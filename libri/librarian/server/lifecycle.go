package server

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // pprof doc calls for black import
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	cbackoff "github.com/cenkalti/backoff"
	cerrors "github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/comm"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	postListenNotifyWait = 100 * time.Millisecond
	maxConcurrentStreams = 128
)

var (
	backoffMaxElapsedTime = 60 * time.Second
)

const (
	// LoggerPortKey is the logger key used for address ports.
	LoggerPortKey = "port"

	// LoggerSeeds is the logger key used for the seeds of a bootstrap operation.
	LoggerSeeds = "seeds"

	// LoggerNBootstrappedPeers is the logger key used for the number of peers found
	// during a bootstrap operation.
	LoggerNBootstrappedPeers = "n_peers"
)

var errInsufficientBootstrappedPeers = errors.New("failed to bootstrap enough other peers")

// Start is the entry point for a Librarian server. It bootstraps peers for the Librarians's
// routing table and then begins listening for and handling requests. It notifies the up channel
// just before
func Start(logger *zap.Logger, config *Config, up chan *Librarian) error {
	// create librarian
	l, err := NewLibrarian(config, logger)
	if err != nil {
		return err
	}

	errs := make(chan error, 2)

	// populate routing table
	bootstrapped := make(chan struct{})
	go func() {
		if err := l.bootstrapPeers(config.BootstrapAddrs); err != nil {
			errs <- err
		}
		close(bootstrapped)
	}()

	// start main listening thread
	go func() {
		errs <- l.listenAndServe(up, bootstrapped)
	}()

	return <-errs
}

func (l *Librarian) bootstrapPeers(bootstrapAddrs []*net.TCPAddr) error {
	bootstraps, bootstrapAddrStrs := makeBootstrapPeers(bootstrapAddrs)
	l.logger.Info("beginning peer bootstrap", zap.Strings(LoggerSeeds, bootstrapAddrStrs))

	var intro *introduce.Introduction
	operation := func() error {
		intro = introduce.NewIntroduction(l.peerID, l.orgID, l.apiSelf, l.config.Introduce)
		if err := l.introducer.Introduce(intro, bootstraps); err != nil {
			l.logger.Debug("introduction error", zap.String("error", err.Error()))
			return err
		}
		if !intro.ReachedMin() {
			// error if couldn't find any/enough peers
			l.logger.Debug("not enough bootstrapped peers",
				zap.Int("num_introduced", len(intro.Result.Responded)))
			return errInsufficientBootstrappedPeers
		}
		return nil
	}

	backoff := cbackoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = backoffMaxElapsedTime
	if err := cbackoff.Retry(operation, backoff); err != nil {
		l.logger.Error("encountered fatal error while bootstrapping",
			zap.Error(err),
			zap.Stringer("self_id", l.peerID),
			zap.String("public_name", l.config.PublicName),
		)
		return err
	}

	// add bootstrapped peers to routing table
	var prevAddress string
	for _, p := range intro.Result.Responded {
		q, exists := l.rt.Get(p.ID())
		if exists {
			prevAddress = q.Address().String()
		}
		status := l.rt.Push(p)
		fields := []zapcore.Field{
			zap.Stringer("peer_id", p.ID()),
			zap.Stringer("push_status", status),
			zap.Stringer("address", p.Address()),
		}
		if exists {
			fields = append(fields, zap.String("prev_address", prevAddress))
		}
		l.logger.Debug("bootstrapped peer added", fields...)
	}
	l.logger.Info("bootstrapped peers",
		zap.Int(LoggerNBootstrappedPeers, len(intro.Result.Responded)),
		zap.Int("routing_table_n_peers", l.rt.NumPeers()),
		zap.Int("routing_table_n_buckets", l.rt.NumBuckets()),
	)
	return nil
}

func makeBootstrapPeers(bootstrapAddrs []*net.TCPAddr) (
	[]peer.Peer, []string) {
	peers, addrStrs := make([]peer.Peer, 0), make([]string, 0)
	for i, bootstrap := range bootstrapAddrs {
		dummyIDStr := fmt.Sprintf("bootstrap-seed%02d", i)
		peers = append(peers, peer.New(nil, dummyIDStr, bootstrap))
		addrStrs = append(addrStrs, bootstrap.String())
	}
	return peers, addrStrs
}

func (l *Librarian) listenAndServe(up chan *Librarian, bootstrapped chan struct{}) error {
	s := grpc.NewServer(
		grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor),
		grpc.UnaryInterceptor(grpc_prometheus.UnaryServerInterceptor),
		grpc.MaxConcurrentStreams(maxConcurrentStreams),
	)

	api.RegisterLibrarianServer(s, l)
	healthpb.RegisterHealthServer(s, l.health)
	if l.config.ReportMetrics {
		grpc_prometheus.Register(s)
		grpc_prometheus.EnableHandlingTimeHistogram()
		l.storageMetrics.register()
		if rec, ok := l.rec.(comm.PromRecorder); ok {
			rec.Register()
		}
	}
	reflection.Register(s)

	// aux routines handle:
	// - (maybe) start Prometheus metrics endpoint
	// - (maybe) start pprof profiler endpoint
	// - listening to SIGTERM (and friends) signals from outside world
	// - sending publications to subscribed peers
	// - document replication
	l.startAuxRoutines(bootstrapped)

	// handle stop signal
	go func() {
		<-l.stop
		l.logger.Info("gracefully stopping server", zap.Int(LoggerPortKey, l.config.LocalPort))
		s.GracefulStop()
		if l.config.ReportMetrics {
			l.storageMetrics.unregister()
			if rec, ok := l.rec.(comm.PromRecorder); ok {
				rec.Unregister()
			}
		}
		close(l.stopped)
	}()

	// notify up channel shortly after starting to serve requests
	go func() {
		time.Sleep(postListenNotifyWait)
		l.logger.Info("listening for requests", zap.Int(LoggerPortKey, l.config.LocalPort))

		// set top-level health status
		l.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

		up <- l
	}()

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", l.config.LocalPort))
	if err != nil {
		l.logger.Error("failed to listen", zap.Error(err))
		return err
	}
	if err := s.Serve(lis); err != nil {
		if strings.Contains(err.Error(), "use of closed network connection") {
			return nil
		}
		l.logger.Error("failed to serve", zap.Error(err))
		return err
	}
	return nil
}

func (l *Librarian) startAuxRoutines(bootstrapped chan struct{}) {
	if l.config.ReportMetrics {
		go func() {
			if err := l.metrics.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				l.logger.Error("error serving Prometheus metrics", zap.Error(err))
				cerrors.MaybePanic(l.Close()) // don't try to recover from Close error
			}
		}()
	}

	if l.config.Profile {
		go func() {
			profilerAddr := fmt.Sprintf(":%d", l.config.LocalProfilerPort)
			if err := http.ListenAndServe(profilerAddr, nil); err != nil {
				l.logger.Error("error serving profiler", zap.Error(err))
				cerrors.MaybePanic(l.Close()) // don't try to recover from Close error
			}
		}()
	}

	// handle stop stopSignals from outside world
	stopSignals := make(chan os.Signal, 3)
	signal.Notify(stopSignals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	go func() {
		<-stopSignals
		cerrors.MaybePanic(l.Close()) // don't try to recover from Close error
	}()

	// long-running goroutine managing subscriptions from other peers
	go l.subscribeFrom.Fanout()

	// long-running goroutine managing subscriptions to other peers
	go func() {
		// wait until have bootstrapped peers
		<-bootstrapped
		if err := l.subscribeTo.Begin(); err != nil {
			l.logger.Error("fatal subscriptionTo error", zap.Error(err))
			cerrors.MaybePanic(l.Close()) // don't try to recover from Close error
		}
	}()

	// long-running goroutine replicating documents
	go func() {
		// wait until have bootstrapped peers
		<-bootstrapped
		time.Sleep(60 * time.Second)
		if err := l.replicator.Start(); err != nil {
			l.logger.Error("fatal replicator error", zap.Error(err))
			cerrors.MaybePanic(l.Close()) // don't try to recover from Close error
		}
	}()
}

// StopAuxRoutines ends the replicator and subscriptions auxiliary routines.
func (l *Librarian) StopAuxRoutines() {
	l.replicator.Stop()
	l.subscribeTo.End()
}

// Close handles cleanup involved in closing down the server.
func (l *Librarian) Close() error {
	l.StopAuxRoutines()

	// send stop signal to listener
	select {
	case <-l.stop: // already closed
	default:
		close(l.stop)
	}

	// end metrics server
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	if err := l.metrics.Shutdown(ctx); err != nil {
		if err == context.DeadlineExceeded {
			if err := l.metrics.Close(); err != nil {
				return err
			}
		}
	}
	cancel()

	// close all client connections
	if err := l.clients.CloseAll(); err != nil {
		return err
	}

	// wait for server to stop
	<-l.stopped

	// close the DB
	l.db.Close()

	return nil
}

// CloseAndRemove cleans up and removes any local state from the server.
func (l *Librarian) CloseAndRemove() error {
	err := l.Close()
	if err != nil {
		return err
	}
	return os.RemoveAll(l.config.DataDir)
}

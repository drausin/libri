package server

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	cbackoff "github.com/cenkalti/backoff"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"os/signal"
	"syscall"
)

const (
	postListenNotifyWait = 100 * time.Millisecond
	backoffMaxElapsedTime = 5 * time.Second
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

var errNoBootstrappedPeers = errors.New("failed to bootstrap any other peers")

// Start is the entry point for a Librarian server. It bootstraps peers for the Librarians's
// routing table and then begins listening for and handling requests. It notifies the up channel
// just before
func Start(logger *zap.Logger, config *Config, up chan *Librarian) error {
	// create librarian
	l, err := NewLibrarian(config, logger)
	if err != nil {
		return err
	}

	// populate routing table
	if err := l.bootstrapPeers(config.BootstrapAddrs); err != nil {
		return err
	}

	// start main listening thread
	if err := l.listenAndServe(up); err != nil {
		return err
	}

	return nil
}

func (l *Librarian) bootstrapPeers(bootstrapAddrs []*net.TCPAddr) error {
	bootstraps, bootstrapAddrStrs := makeBootstrapPeers(bootstrapAddrs, l.config.PublicAddr)
	l.logger.Info("beginning peer bootstrap", zap.Strings(LoggerSeeds, bootstrapAddrStrs))

	var intro *introduce.Introduction
	operation := func() error {
		intro = introduce.NewIntroduction(l.selfID, l.apiSelf, l.config.Introduce)
		if err := l.introducer.Introduce(intro, bootstraps); err != nil {
			l.logger.Debug("introduction error", zap.String("error", err.Error()))
			return err
		}
		if !l.config.isBootstrap() && len(intro.Result.Responded) == 0 {
			// if we're not a libri bootstrap peer, error if couldn't find any
			l.logger.Debug("no bootstrapped peers",
				zap.String("error", errNoBootstrappedPeers.Error()),
			)
			return errNoBootstrappedPeers
		}
		return nil
	}

	backoff := cbackoff.NewExponentialBackOff()
	backoff.MaxElapsedTime = backoffMaxElapsedTime
	if err := cbackoff.Retry(operation, backoff); err != nil {
		l.logger.Error("encountered fatal error while bootstrapping",
			zap.Error(err),
			zap.Stringer("self_id", l.selfID),
			zap.String("public_name", l.config.PublicName),
		)
		return err
	}

	// add bootstrapped peers to routing table
	for _, p := range intro.Result.Responded {
		l.rt.Push(p)
	}
	l.logger.Info("bootstrapped peers",
		zap.Int(LoggerNBootstrappedPeers, len(intro.Result.Responded)))
	return nil
}

func makeBootstrapPeers(bootstrapAddrs []*net.TCPAddr, selfPublicAddr fmt.Stringer) (
	[]peer.Peer, []string) {
	peers, addrStrs := make([]peer.Peer, 0), make([]string, 0)
	for i, bootstrap := range bootstrapAddrs {
		if bootstrap.String() != selfPublicAddr.String() {
			dummyIDStr := fmt.Sprintf("bootstrap-seed%02d", i)
			conn := api.NewConnector(bootstrap)
			peers = append(peers, peer.New(nil, dummyIDStr, conn))
			addrStrs = append(addrStrs, bootstrap.String())
		}
	}
	return peers, addrStrs
}

func (l *Librarian) listenAndServe(up chan *Librarian) error {
	lis, err := net.Listen("tcp", l.config.LocalAddr.String())
	if err != nil {
		l.logger.Error("failed to listen", zap.Error(err))
		return err
	}

	s := grpc.NewServer()
	api.RegisterLibrarianServer(s, l)
	healthpb.RegisterHealthServer(s, l.health)
	reflection.Register(s)

	// handle stop signal
	go func() {
		<-l.stop
		l.logger.Info("gracefully stopping server", zap.Int(LoggerPortKey,
			l.config.LocalAddr.Port))
		s.GracefulStop()
	}()

	// handle stop stopSignals from outside world
	stopSignals := make(chan os.Signal, 3)
	signal.Notify(stopSignals, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	go func() {
		<-stopSignals
		if err := l.Close(); err != nil {
			// don't try to recover
			panic(err)
		}
	}()

	// long-running goroutine managing subscriptions from other peers
	go l.subscribeFrom.Fanout()

	// long-running goroutine managing subscriptions to other peers
	go func() {
		if err := l.subscribeTo.Begin(); err != nil && !l.config.isBootstrap() {
			l.logger.Error("fatal subscriptionTo error", zap.Error(err))
			if err := l.Close(); err != nil {
				panic(err) // don't try to recover from Close error
			}

		}
	}()

	// notify up channel shortly after starting to serve requests
	go func() {
		time.Sleep(postListenNotifyWait)
		l.logger.Info("listening for requests", zap.Int(LoggerPortKey,
			l.config.LocalAddr.Port))

		// set top-level health status
		l.health.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

		up <- l
	}()

	if err := s.Serve(lis); err != nil {
		if strings.Contains(fmt.Sprintf("%s", err.Error()), "use of closed network connection") {
			return nil
		}
		l.logger.Error("failed to serve", zap.Error(err))
		return err
	}
	return nil
}

// Close handles cleanup involved in closing down the server.
func (l *Librarian) Close() error {

	// end subscriptions to other peers
	l.subscribeTo.End()

	// send stop signal to listener
	select {
	case <-l.stop: // already closed
	default:
		close(l.stop)
	}

	// disconnect from peers in routing table
	if err := l.rt.Disconnect(); err != nil {
		return err
	}

	// save routing table state
	if err := l.rt.Save(l.serverSL); err != nil {
		return err
	}

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

package server

import (
	"fmt"
	"net"
	"os"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"go.uber.org/zap"
	"github.com/pkg/errors"
)

var (
	LoggerPortKey = "port"
)



// Start is the entry point for a Librarian server. It bootstraps peers for the Librarians's
// routing table and then begins listening for and handling requests.
func Start(config *Config, logger *zap.Logger) error {
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
	if err := l.listenAndServe(); err != nil {
		return err
	}

	return nil
}

func (l *Librarian) bootstrapPeers(bootstrapAddrs []*net.TCPAddr) error {
	intro := introduce.NewIntroduction(l.SelfID, l.apiSelf, introduce.NewDefaultParameters())
	err := l.introducer.Introduce(intro, makeBootstrapPeers(bootstrapAddrs))
	if err != nil {
		l.logger.Error("encountered fatal error while bootsrapping", zap.Error(err))
		return err
	}
	if len(intro.Result.Responded) == 0 {
		err := errors.New("failed to bootstrap any other peers")
		l.logger.Error("failed to bootstrap any other peers")
		return err
	}

	// add bootstrapped peers to routing table
	for _, p := range intro.Result.Responded {
		l.rt.Push(p)
	}
	return nil
}

func makeBootstrapPeers(bootstrapAddrs []*net.TCPAddr) []peer.Peer {
	peers := make([]peer.Peer, len(bootstrapAddrs))
	for i, bootstrap := range bootstrapAddrs {
		dummyIDStr := fmt.Sprintf("bootstrap-seed%02d", i)
		peers[i] = peer.New(nil, dummyIDStr, peer.NewConnector(bootstrap))
	}
	return peers
}

func (l *Librarian) listenAndServe() error {
	lis, err := net.Listen("tcp", l.Config.LocalAddr.String())
	if err != nil {
		l.logger.Error("failed to listen", zap.Error(err))
		return err
	}

	s := grpc.NewServer()
	api.RegisterLibrarianServer(s, l)
	reflection.Register(s)

	l.logger.Info("listening for requests", zap.Int(LoggerPortKey, l.Config.LocalAddr.Port))
	if err := s.Serve(lis); err != nil {
		l.logger.Error("failed to serve", zap.Error(err))
		return nil
	}
	return nil
}

// Close handles cleanup involved in closing down the server.
func (l *Librarian) Close() error {
	if err := l.rt.Disconnect(); err != nil {
		return err
	}
	if err := l.rt.Save(l.serverSL); err != nil {
		return err
	}
	l.db.Close()

	return nil
}

// CloseAndRemove cleans up and removes any local state from the server.
func (l *Librarian) CloseAndRemove() error {
	err := l.Close()
	if err != nil {
		return err
	}
	return os.RemoveAll(l.Config.DataDir)
}

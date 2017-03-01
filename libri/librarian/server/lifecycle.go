package server

import (
	"os"
	"net"
	"log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"fmt"
	"github.com/drausin/libri/libri/librarian/server/introduce"
	"errors"
)


// Start is the main entry point for a Librarian server. It bootstraps peers for the Librarians's
// routing table and then begins listening for and handling requests.
func Start(config *Config) error {

	// create librarian
	l, err := NewLibrarian(config)
	if err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	// populate routing table
	if err := l.bootstrapPeers(config.BootstrapAddrs); err != nil {
		log.Fatalf("%v", err)
	}

	// start main listening thread
	l.listenAndServe()

	return nil
}

func (l *Librarian) bootstrapPeers(bootstrapAddrs []*net.TCPAddr) error {
	intro := introduce.NewIntroduction(l.SelfID, l.apiSelf, introduce.NewDefaultParameters())
	err := l.introducer.Introduce(intro, makeBootstrapPeers(bootstrapAddrs))
	if err != nil {
		return fmt.Errorf("encountered fatal error while bootsrapping: %v", err)
	}
	if len(intro.Result.Responded) == 0 {
		return errors.New("failed to bootstrap any other peers -> exiting")
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

func (l *Librarian) listenAndServe() {
	lis, err := net.Listen("tcp", l.Config.RPCLocalAddr.String())
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	api.RegisterLibrarianServer(s, l)
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
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

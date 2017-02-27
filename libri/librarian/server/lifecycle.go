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
)


func Start(config *Config) error {

	// create librarian
	l, err := NewLibrarian(config)
	if err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	// populate routing table
	l.bootstrapPeers(config.BootstrapAddrs)

	// start main listening thread
	l.listenAndServe()

	return nil
}

func (l *Librarian) bootstrapPeers(bootstrapAddrs []*net.TCPAddr) {
	intro := introduce.NewIntroduction(l.SelfID, l.apiSelf, introduce.NewDefaultParameters())
	l.introducer.Introduce(intro, makeBootstrapPeers(bootstrapAddrs))
	if intro.Errored() {
		log.Fatalf("encountered fatal error while bootsrapping: %v", intro.Result.FatalErr)
	}
	if len(intro.Result.Responded) == 0 {
		log.Fatal("failed to bootstrap any other peers -> exiting.")
	}
}

func makeBootstrapPeers(bootstrapAddrs []*net.TCPAddr) []peer.Peer {
	peers := make([]peer.Peer, len(bootstrapAddrs))
	for i, bootstrap := range bootstrapAddrs {
		dummyIdStr := fmt.Sprintf("bootstrap-seed%02d", i)
		peers[i] = peer.New(nil, dummyIdStr, peer.NewConnector(bootstrap))
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

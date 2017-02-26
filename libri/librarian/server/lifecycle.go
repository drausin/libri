package server

import (
	"os"
	"net"
	"log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/peer"
	"time"
)

var (
	// number of peers to request per Introduce call
	numPeersPerIntroduction = uint(16)

	// number of recursive Introduce iterations
	numIntroduceIterations = uint(2)

	// timeout for each bootstrap Introduce query
	bootstrapQueryTimeout = 5 * time.Second

)

func Start(config *Config) error {

	// create librarian
	l, err := NewLibrarian(config)
	if err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}
	// start server listener
	l.listenAndServe()

	// bootstrap peers

	return nil
}

func (l *Librarian) bootstrapPeers(bootstrapAddrs []*net.TCPAddr) {
	for _, a := range bootstrapAddrs {
		conn := peer.NewConnector(a)
		_, err := conn.Connect()
		if err != nil {
			log.Printf("unable to connect to bootstrap peer %v", a.String())
			continue
		}
	}
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

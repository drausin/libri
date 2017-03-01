package main

import (
	"log"
	"net"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	lib, err := server.NewLibrarian(server.DefaultConfig(0))
	if err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	lis, err := net.Listen("tcp", lib.Config.RPCLocalAddr.String())
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	api.RegisterLibrarianServer(s, lib)
	reflection.Register(s)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

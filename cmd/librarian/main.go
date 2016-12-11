package main

import (
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/drausin/libri/librarian/api"
	"github.com/drausin/libri/librarian/server"
)

func main() {
	lib, err := server.NewLibrarian()
	if err != nil {
		log.Fatalf("failed to initialize: %v", err)
	}

	lis, err := net.Listen("tcp", lib.ServerConfig.RPCAddr.String())
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

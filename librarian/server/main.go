package main

import (
	"log"
	"net"
	"crypto/rand"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"github.com/drausin/libri/pkg/api"
	"google.golang.org/grpc/reflection"
)

type librarian struct {
	// Config holds the configuration parameters of the server
	ServerConfig *Config

	// NodeID is the random 256-bit identification number of this node in the hash table
	NodeID       []byte
}

func New() (*librarian, error) {
	nodeID, err := generateNodeID()
	if err != nil {
		return nil, err
	}

	return &librarian{
		ServerConfig: DefaultConfig(),
		NodeID: nodeID,
	}, nil
}

// Generate a 256-bit random node ID
func generateNodeID() ([]byte, error) {
	nodeID := make([]byte, 32)
	_, err := rand.Read(nodeID); if err != nil {
		return nil, err
	}
	return nodeID, nil
}

func (l *librarian) Ping(ctx context.Context, rq *api.PingRequest) (*api.PingResponse, error) {
	return &api.PingResponse{Message: "pong"}, nil
}

func (l *librarian) Identify(ctx context.Context, rq *api.IdentityRequest) (*api.IdentityResponse, error) {
	return &api.IdentityResponse{
		NodeName: l.ServerConfig.NodeName,
		NodeId:   l.NodeID,
	}, nil
}

func main() {
	lib, err := New()
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

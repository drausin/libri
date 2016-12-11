package main

import (
	"log"
	"encoding/base64"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"github.com/drausin/libri/pkg/api"
)

const (
	address = "localhost:11000"
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := api.NewLibrarianClient(conn)

	// Ping the server
	r1, err := c.Ping(context.Background(), &api.PingRequest{})
	if err != nil {
		log.Fatalf("could not ping: %v", err)
	}
	log.Printf("Server: %s", r1.Message)

	r2, err := c.Identify(context.Background(), &api.IdentityRequest{})
	if err != nil {
		log.Fatalf("could not ping: %v", err)
	}
	log.Printf("Node name: %s", r2.NodeName)
	log.Printf("Node ID: %v", base64.URLEncoding.EncodeToString(r2.NodeId))
}
package main

import (
	"encoding/base64"
	"log"

	"github.com/drausin/libri/libri/librarian/api"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	address = "localhost:11000"
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
		panic(err)
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
	log.Printf("Peer name: %s", r2.PeerName)
	log.Printf("Peer ID: %v", base64.URLEncoding.EncodeToString(r2.PeerId))
}

package main

import (
	"log"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"github.com/drausin/libri/pkg/api"
)

const (
	address = "localhost:50051"
)

func main() {
	// Set up a connection to the server.
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := api.NewLibrarianClient(conn)

	// Contact the server and print out its response.
	r, err := c.Ping(context.Background(), &api.PingRequest{})
	if err != nil {
		log.Fatalf("could not ping: %v", err)
	}
	log.Printf("Server: %s", r.Message)
}
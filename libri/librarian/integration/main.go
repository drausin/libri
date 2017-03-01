package main

import (
	"log"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
	"github.com/drausin/libri/libri/librarian/server/ecid"
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
	c := api.NewLibrarianClient(conn)

	// Ping the server
	r1, err := c.Ping(context.Background(), &api.PingRequest{})
	if err != nil {
		log.Fatalf("could not ping: %v", err)
	}
	log.Printf("Server: %s", r1.Message)

	r2, err := c.Introduce(context.Background(), &api.IntroduceRequest{
		Metadata: api.NewRequestMetadata(ecid.NewRandom()),
	})
	if err != nil {
		log.Fatalf("could not ping: %v", err)
	}
	log.Printf("Peer name: %s", r2.Self.PeerName)
	peerPubKey, err := ecid.FromPublicKeyBytes(r2.Metadata.PubKey)
	if err != nil {
		log.Fatalf("could not read public key: %v", err)
	}
	log.Printf("Peer ID: %v", cid.FromPublicKey(peerPubKey))

	err = conn.Close()
	if err != nil {
		panic(err)
	}
}

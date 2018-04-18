package api

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type Endpoint int

const (
	All Endpoint = iota - 1
	Introduce
	Find
	Store
	Verify
	Get
	Put
	Subscribe
)

var (
	Endpoints = []Endpoint{Introduce, Find, Store, Verify, Get, Put, Subscribe}
)

func (e Endpoint) String() string {
	switch e {
	case All:
		return "All"
	case Introduce:
		return "Introduce"
	case Find:
		return "Find"
	case Store:
		return "Store"
	case Verify:
		return "Verify"
	case Get:
		return "Get"
	case Put:
		return "Put"
	case Subscribe:
		return "Subscribe"
	default:
		panic("unknown endpoint")
	}
}

// These interfaces split up the methods of LibrarianClient, mostly to allow for narrow interface
// usage and testing.

// Introducer issues Introduce queries.
type Introducer interface {
	// Introduce identifies the node by name and ID.
	Introduce(ctx context.Context, in *IntroduceRequest, opts ...grpc.CallOption) (
		*IntroduceResponse, error)
}

// Finder issues Find queries.
type Finder interface {
	// Find returns the value for a key or the closest peers to it.
	Find(ctx context.Context, in *FindRequest, opts ...grpc.CallOption) (*FindResponse, error)
}

// Storer issues Store queries.
type Storer interface {
	// Store stores a value in a given key.
	Store(ctx context.Context, in *StoreRequest, opts ...grpc.CallOption) (*StoreResponse,
		error)
}

// Verifier issues Verify queries.
type Verifier interface {
	// Verify verifies that a peer has a given value.
	Verify(ctx context.Context, in *VerifyRequest, opts ...grpc.CallOption) (*VerifyResponse, error)
}

// Getter issues Get queries.
type Getter interface {
	// Get retrieves a value, if it exists.
	Get(ctx context.Context, in *GetRequest, opts ...grpc.CallOption) (*GetResponse, error)
}

// Putter issues Put queries.
type Putter interface {
	// Put stores a value.
	Put(ctx context.Context, in *PutRequest, opts ...grpc.CallOption) (*PutResponse, error)
}

// PutterGetter issues Put and Get queries.
type PutterGetter interface {
	Getter
	Putter
}

// Subscriber issues Subscribe queries.
type Subscriber interface {
	// Subscribe subscribes to a defined publication stream.
	Subscribe(ctx context.Context, in *SubscribeRequest, opts ...grpc.CallOption) (
		Librarian_SubscribeClient, error)
}

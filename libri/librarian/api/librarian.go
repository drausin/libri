package api

import (
	"google.golang.org/grpc"
	"golang.org/x/net/context"
)

// These interfaces split up the methods of LibrarianClient, mostly for simplifying testing.

// Pinger issues Ping queries.
type Pinger interface {
	// Ping confirms simple request/response connectivity.
	Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error)
}

// Introducer issues Introduce queries.
type Introducer interface {
	// Identify identifies the node by name and ID.
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


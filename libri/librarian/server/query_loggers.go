package server

import (
	"github.com/drausin/libri/libri/librarian/server/peer"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/api"
)

type RequestLogger interface {
	// Log records a query outcome from a given context.
	Log(meta api.RequestMetadata, rt routing.Table, o peer.Outcome) error
}

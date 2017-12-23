package client

import (
	"github.com/drausin/libri/libri/librarian/api"
)

// IntroducerCreator creates api.Introducers.
type IntroducerCreator interface {
	// Create creates an api.Introducer from the api.Connector.
	Create(address string) (api.Introducer, error)
}

type introducerCreator struct {
	clients Pool
}

// NewIntroducerCreator creates a new IntroducerCreator.
func NewIntroducerCreator(clients Pool) IntroducerCreator {
	return &introducerCreator{clients}
}

func (c *introducerCreator) Create(address string) (api.Introducer, error) {
	lc, err := c.clients.Get(address)
	if err != nil {
		return nil, err
	}
	return lc.(api.Introducer), nil
}

// FinderCreator creates api.Finders.
type FinderCreator interface {
	// Create creates an api.Finder from the api.Connector.
	Create(address string) (api.Finder, error)
}

type finderCreator struct {
	clients Pool
}

// NewFinderCreator creates a new FinderCreator.
func NewFinderCreator(clients Pool) FinderCreator {
	return &finderCreator{clients}
}

func (c *finderCreator) Create(address string) (api.Finder, error) {
	lc, err := c.clients.Get(address)
	if err != nil {
		return nil, err
	}
	return lc.(api.Finder), nil
}

// VerifierCreator creates api.Verifiers.
type VerifierCreator interface {
	// Create creates an api.Verifier from the api.Connector.
	Create(address string) (api.Verifier, error)
}

type verifierCreator struct {
	clients Pool
}

// NewVerifierCreator creates a new FinderCreator.
func NewVerifierCreator(clients Pool) VerifierCreator {
	return &verifierCreator{clients}
}

func (c *verifierCreator) Create(address string) (api.Verifier, error) {
	lc, err := c.clients.Get(address)
	if err != nil {
		return nil, err
	}
	return lc.(api.Verifier), nil
}

// StorerCreator creates api.Storers.
type StorerCreator interface {
	// Create creates an api.Storer from the api.Connector.
	Create(address string) (api.Storer, error)
}

type storerCreator struct {
	clients Pool
}

// NewStorerCreator creates a new StorerCreator.
func NewStorerCreator(clients Pool) StorerCreator {
	return &storerCreator{clients}
}

func (c *storerCreator) Create(address string) (api.Storer, error) {
	lc, err := c.clients.Get(address)
	if err != nil {
		return nil, err
	}
	return lc.(api.Storer), nil
}

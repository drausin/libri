package client

import (
	"github.com/drausin/libri/libri/librarian/api"
)

// IntroducerCreator creates api.Introducers.
type IntroducerCreator interface {
	// Create creates an api.Introducer from the api.Connector.
	Create(conn api.Connector) (api.Introducer, error)
}

type introducerCreator struct{}

// NewIntroducerCreator creates a new IntroducerCreator.
func NewIntroducerCreator() IntroducerCreator {
	return &introducerCreator{}
}

func (*introducerCreator) Create(c api.Connector) (api.Introducer, error) {
	lc, err := c.Connect()
	if err != nil {
		return nil, err
	}
	return lc.(api.Introducer), nil
}

// FinderCreator creates api.Finders.
type FinderCreator interface {
	// Create creates an api.Finder from the api.Connector.
	Create(conn api.Connector) (api.Finder, error)
}

type finderCreator struct{}

// NewFinderCreator creates a new FinderCreator.
func NewFinderCreator() FinderCreator {
	return &finderCreator{}
}

func (*finderCreator) Create(c api.Connector) (api.Finder, error) {
	lc, err := c.Connect()
	if err != nil {
		return nil, err
	}
	return lc.(api.Finder), nil
}

// StorerCreator creates api.Storers.
type StorerCreator interface {
	// Create creates an api.Storer from the api.Connector.
	Create(conn api.Connector) (api.Storer, error)
}

type storerCreator struct{}

// NewStorerCreator creates a new StorerCreator.
func NewStorerCreator() StorerCreator {
	return &storerCreator{}
}

func (*storerCreator) Create(c api.Connector) (api.Storer, error) {
	lc, err := c.Connect()
	if err != nil {
		return nil, err
	}
	return lc.(api.Storer), nil
}

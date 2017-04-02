package author

import "github.com/drausin/libri/libri/librarian/api"

type LibrarianBalancer interface {
	Next() api.LibrarianClient
}



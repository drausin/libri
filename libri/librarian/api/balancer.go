package api

// ClientBalancer load balances between a collection of LibrarianClients.
type ClientBalancer interface {
	Next() LibrarianClient
}

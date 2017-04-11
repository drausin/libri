package author

import (
	"net"

	"github.com/drausin/libri/libri/author/io/print"
	"go.uber.org/zap/zapcore"
	"github.com/drausin/libri/libri/author/io/publish"
)

// Config is used to configure an Author.
type Config struct {
	Print *print.Parameters

	Publish *publish.Parameters

	// DataDir is the directory on the local machine where the state and output of all the
	// clients running on that machine are stored. For example,
	//
	//	data
	//	└── client-fc593d11
	//	    └── db
	//	└── client-f829ef46
	//	    └── db
	//	└── client-765079fb
	//	    └── db
	//
	DataDir string

	// DbDir is the local directory where this node's DB state is stored.
	DbDir string

	KeychainDir string

	// LibrarianAddrs is a list of public addresses of Librarian servers to issue request to.
	LibrarianAddrs []*net.TCPAddr

	// LogLevel is the log level
	LogLevel zapcore.Level
}

package author

import (
	"net"
	"go.uber.org/zap/zapcore"
)

// Config is used to configure an Author.
type Config struct {
	// DataDir is the directory on the local machine where the state and output of all the
	// peers running on that machine are stored. For example,
	//
	//	data
	//	└── peer-fc593d11
	//	    └── db
	//	└── peer-f829ef46
	//	    └── db
	//	└── peer-765079fb
	//	    └── db
	//
	DataDir string

	// DbDir is the local directory where this node's DB state is stored.
	DbDir string

	// LibrarianAddrs is a list of public addresses of Librarian servers to issue request to.
	LibrarianAddrs []*net.TCPAddr

	// LogLevel is the log level
	LogLevel zapcore.Level
}


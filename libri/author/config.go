package author

import (
	"net"
	"os"
	"path/filepath"

	"github.com/drausin/libri/libri/author/io/print"
	"github.com/drausin/libri/libri/author/io/publish"
	"github.com/drausin/libri/libri/librarian/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	// DefaultLibrarianPort is the default port of the librarian server.
	DefaultLibrarianPort = 11000

	// DefaultLibrarianIP is the default IP of of the librarian server.
	DefaultLibrarianIP = "localhost"

	// DefaultLogLevel is the default log level to use.
	DefaultLogLevel = zap.InfoLevel

	// DataSubdir is the name of the data directory.
	DataSubdir = "author-data"

	// DBSubDir is the default DB subdirectory within the data dir.
	DBSubDir = "db"

	// KeychainSubDir is the default DB subdirectory within the data dir.
	KeychainSubDir = "keychain"
)

// Config is used to configure an Author.
type Config struct {
	// DataDir is the directory on the local machine where the state and output of the
	// client is stored.
	DataDir string

	// DbDir is the local directory where this node's DB state is stored.
	DbDir string

	// KeychainDir is the local directory where the author keys are stored.
	KeychainDir string

	// LibrarianAddrs is a list of public addresses of Librarian servers to issue request to.
	LibrarianAddrs []*net.TCPAddr

	// Print defines parameters for printing pages to local storage.
	Print *print.Parameters

	// Publish defines parameters for publishing pages to libri.
	Publish *publish.Parameters

	// LogLevel is the log level
	LogLevel zapcore.Level
}

// NewDefaultConfig returns a reasonable default author configuration.
func NewDefaultConfig() *Config {
	config := &Config{}

	// set defaults via zero values; in cases where the config B depends on config A, config A
	// should be set before config B
	config.WithDefaultDataDir()
	config.WithDefaultDBDir()
	config.WithDefaultKeychainDir()
	config.WithDefaultLibrarianAddrs()
	config.WithDefaultPrint()
	config.WithDefaultPublish()
	config.WithDefaultLogLevel()

	return config
}

// WithDataDir sets the data dir to the given value or the default if the given value is empty.
func (c *Config) WithDataDir(dataDir string) *Config {
	if dataDir == "" {
		return c.WithDefaultDataDir()
	}
	c.DataDir = dataDir
	return c
}

// WithDefaultDataDir sets the data dir to a 'data' subdir of the current working directory..
func (c *Config) WithDefaultDataDir() *Config {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	c.DataDir = filepath.Join(cwd, DataSubdir)
	return c
}

// WithDBDir sets the DB dir to the given value or the default if the given value is empty.
func (c *Config) WithDBDir(dbDir string) *Config {
	if dbDir == "" {
		return c.WithDefaultDBDir()
	}
	c.DbDir = dbDir
	return c
}

// WithDefaultDBDir sets the DB dir to a local name subdir of the data dir.
func (c *Config) WithDefaultDBDir() *Config {
	c.DbDir = filepath.Join(c.DataDir, DBSubDir)
	return c
}

// WithKeychainDir sets the keychain dir to the given value or the default if the given value is
// empty.
func (c *Config) WithKeychainDir(keychainDir string) *Config {
	if keychainDir == "" {
		return c.WithDefaultKeychainDir()
	}
	c.KeychainDir = keychainDir
	return c
}

// WithDefaultKeychainDir sets the keychain dir to a local name subdir of the data dir.
func (c *Config) WithDefaultKeychainDir() *Config {
	c.KeychainDir = filepath.Join(c.DataDir, KeychainSubDir)
	return c
}

// WithLibrarianAddrs sets the librarian addresses to the given value or the default if the given
// value is empty.
func (c *Config) WithLibrarianAddrs(librarianAddrs []*net.TCPAddr) *Config {
	if librarianAddrs == nil {
		return c.WithDefaultLibrarianAddrs()
	}
	c.LibrarianAddrs = librarianAddrs
	return c
}

// WithDefaultLibrarianAddrs sets the librarian addresses to a single address of the default IP
// and port.
func (c *Config) WithDefaultLibrarianAddrs() *Config {
	c.LibrarianAddrs = []*net.TCPAddr{
		server.ParseAddr(DefaultLibrarianIP, DefaultLibrarianPort),
	}
	return c
}

// WithPrint sets the Print parameters to the given value or the default if it is nil.
func (c *Config) WithPrint(params *print.Parameters) *Config {
	if params == nil {
		return c.WithDefaultPrint()
	}
	c.Print = params
	return c
}

// WithDefaultPrint sets the Print parameters to the default values specified in the
// print package.
func (c *Config) WithDefaultPrint() *Config {
	c.Print = print.NewDefaultParameters()
	return c
}

// WithPublish sets the Publish parameters to the given value or the default if it is nil.
func (c *Config) WithPublish(params *publish.Parameters) *Config {
	if params == nil {
		return c.WithDefaultPublish()
	}
	c.Publish = params
	return c
}

// WithDefaultPublish sets the Publish parameters to the default values specified in the
// publish package.
func (c *Config) WithDefaultPublish() *Config {
	c.Publish = publish.NewDefaultParameters()
	return c
}

// WithLogLevel sets the log level to the given value, though this doesn't have any direct effect
// on the creation of the logger instance.
func (c *Config) WithLogLevel(logLevel zapcore.Level) *Config {
	if logLevel == 0 {
		return c.WithDefaultLogLevel()
	}
	c.LogLevel = logLevel
	return c
}

// WithDefaultLogLevel sets the log level to INFO.
func (c *Config) WithDefaultLogLevel() *Config {
	c.LogLevel = DefaultLogLevel
	return c
}

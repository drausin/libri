package cmd

import (
	"io/ioutil"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/logging"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"path/filepath"
)

func TestAuthorGetter_get_ok(t *testing.T) {
	keychainDir, err := ioutil.TempDir("", "test-author")
	assert.Nil(t, err)
	config := author.NewDefaultConfig().
		WithDataDir(keychainDir).
		WithDefaultDBDir().
		WithDefaultKeychainDir()
	logger1 := server.NewDevInfoLogger()

	ag := authorGetterImpl{
		acg: &fixedAuthorConfigGetter{
			config: config,
			logger: logger1,
		},
	}
	authorKeys, selfReaderKeys := keychain.New(1), keychain.New(1)

	author2, logger2, err := ag.get(authorKeys, selfReaderKeys)
	assert.Nil(t, err)
	assert.NotNil(t, author2)
	assert.Equal(t, logger1, logger2)

	assert.Nil(t, os.RemoveAll(keychainDir))
	assert.Nil(t, os.RemoveAll(config.DataDir))
}

func TestAuthorGetter_get_err(t *testing.T) {
	ag := authorGetterImpl{
		acg: &fixedAuthorConfigGetter{
			err: errors.New("some config get error"),
		},
	}
	authorKeys, selfReaderKeys := keychain.New(1), keychain.New(1)

	author2, logger2, err := ag.get(authorKeys, selfReaderKeys)
	assert.NotNil(t, err)
	assert.Nil(t, author2)
	assert.Nil(t, logger2)
}

func TestAuthorConfigGetter_get_ok(t *testing.T) {
	dataDir, logLevel := "some/data/dir", zap.DebugLevel
	libAddrs := []string{"127.0.0.1:1234", "127.0.0.1:5678"}
	libAddrsArg := strings.Join(libAddrs, " ")
	log.Print(libAddrsArg)
	viper.Set(dataDirFlag, dataDir)
	viper.Set(logLevelFlag, logLevel)
	viper.Set(authorLibrariansFlag, libAddrsArg)
	acg := &authorConfigGetterImpl{}

	config, logger, err := acg.get(authorLibrariansFlag)

	assert.Nil(t, err)
	assert.Equal(t, logLevel, config.LogLevel)
	assert.Equal(t, len(libAddrs), len(config.LibrarianAddrs))
	for i, la := range config.LibrarianAddrs {
		assert.Equal(t, libAddrs[i], la.String())
	}
	assert.NotNil(t, logger)

	cwd, err := os.Getwd()
	assert.Nil(t, err)
	assert.Nil(t, os.RemoveAll(filepath.Join(cwd, dataDir)))
}

func TestAuthorConfigGetter_get_err(t *testing.T) {
	viper.Set(authorLibrariansFlag, "not an address")
	acg := &authorConfigGetterImpl{}

	config, logger, err := acg.get(authorLibrariansFlag)

	assert.NotNil(t, err)
	assert.Nil(t, config)
	assert.NotNil(t, logger) // still should have been created
}

type fixedAuthorConfigGetter struct {
	config *author.Config
	logger *zap.Logger
	err    error
}

func (f *fixedAuthorConfigGetter) get(librariansFlag string) (*author.Config, *zap.Logger, error) {
	return f.config, f.logger, f.err
}

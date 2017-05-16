package cmd

import (
	"go.uber.org/zap"
	"github.com/spf13/cobra"
	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/librarian/server"
	clogging "github.com/drausin/libri/libri/common/logging"
	"log"
	"github.com/drausin/libri/libri/author/keychain"
)

var (
	librarianAddrs []string
	createKeychain bool
	passphase      string
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "test a set of librarian servers",
	Long:  `TODO (drausin) add longer description here`,
}

func init() {
	RootCmd.AddCommand(testCmd)

	testCmd.PersistentFlags().StringVarP(&passphase, "passphrase", "p", "atestpassphrase",
		"keychain passphrase")
	testCmd.PersistentFlags().StringArrayVarP(&librarianAddrs, "librarian-addrs", "a", nil,
		"comma-separated addresses (IPv4:Port) of librarian(s)")
	testCmd.PersistentFlags().StringVarP(&dataDir, "data-dir", "d", "",
		"local data directory")
	testCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "v", zap.InfoLevel.String(),
		"log level")
	testCmd.PersistentFlags().BoolVarP(&createKeychain, "create-keychain", "k", true,
		"create a keychain if one doesn't exist")
}

func getAuthorConfig() (*author.Config, *zap.Logger, error) {
	config := author.NewDefaultConfig().
		WithDataDir(dataDir).
		WithLogLevel(getLogLevel())
	logger := clogging.NewDevLogger(config.LogLevel)
	librarianNetAddrs, err := server.ParseAddrs(librarianAddrs)
	if err != nil {
		logger.Error("unable to parse librarian address", zap.Error(err))
		return nil, logger, err
	}
	config.WithLibrarianAddrs(librarianNetAddrs)

	return config, logger, nil
}

func maybeCreateKeychain(logger *zap.Logger, keychainDir string, keychainAuth string) error {
	missing, err := author.MissingKeychains(keychainDir)
	if err != nil {
		return err
	}
	if !missing {
		return nil
	}

	logger.Info("creating new keychains")
	return author.CreateKeychains(logger, keychainDir, keychainAuth, keychain.StandardScryptN,
		keychain.StandardScryptP)
}

func getAuthor() (*author.Author, *zap.Logger, error) {
	config, logger, err := getAuthorConfig()
	if err != nil {
		return nil, logger, err
	}
	log.Print("about to maybe create keychain")
	if err = maybeCreateKeychain(logger, config.KeychainDir, passphase); err != nil {
		logger.Error("encountered error when creating keychain", zap.Error(err))
		return nil, logger, err
	}
	a, err := author.NewAuthor(config, passphase, logger)
	return a, logger, err
}

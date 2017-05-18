package cmd

import (
	"go.uber.org/zap"
	"github.com/spf13/cobra"
	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/librarian/server"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/spf13/viper"
	"fmt"
)

const (
	passphraseFlag = "passphrase"
	librariansFlag = "librarians"
	createKeychainFlag = "createKeychain"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "test a set of librarian servers",
	Long:  `TODO (drausin) add longer description here`,
}

func init() {
	RootCmd.AddCommand(testCmd)

	testCmd.PersistentFlags().StringP(passphraseFlag, "p", "SamplePassphrase",
		"keychain passphrase")
	testCmd.PersistentFlags().StringArrayP(librariansFlag, "a", nil,
		"comma-separated addresses (IPv4:Port) of librarian(s)")
	testCmd.PersistentFlags().StringP(dataDirFlag, "d", "",
		"local data directory")
	testCmd.PersistentFlags().StringP(logLevelFlag, "v", zap.InfoLevel.String(),
		"log level")
	testCmd.PersistentFlags().BoolP(createKeychainFlag, "k", true,
		"create a keychain if one doesn't exist")

	// bind viper flags
	viper.SetEnvPrefix("LIBRI")   // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()          // read in environment variables that match
	if err := viper.BindPFlags(testCmd.Flags()); err != nil {
		panic(err)
	}
}

func getAuthorConfig() (*author.Config, *zap.Logger, error) {
	config := author.NewDefaultConfig().
		WithDataDir(viper.GetString(dataDirFlag)).
		WithLogLevel(getLogLevel())

	logger := clogging.NewDevLogger(config.LogLevel)
	librarianNetAddrs, err := server.ParseAddrs(viper.GetStringSlice(librariansFlag))
	if err != nil {
		logger.Error("unable to parse librarian address", zap.Error(err))
		return nil, logger, err
	}
	config.WithLibrarianAddrs(librarianNetAddrs)

	logger.Info("test configuration",
		zap.String(librariansFlag, fmt.Sprintf("%v", config.LibrarianAddrs)),
		zap.String(dataDirFlag, config.DataDir),
		zap.Stringer(logLevelFlag, config.LogLevel),
	)
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
	passphrase := viper.GetString(passphraseFlag)
	if err = maybeCreateKeychain(logger, config.KeychainDir, passphrase); err != nil {
		logger.Error("encountered error when creating keychain", zap.Error(err))
		return nil, logger, err
	}
	a, err := author.NewAuthor(config, passphrase, logger)
	return a, logger, err
}

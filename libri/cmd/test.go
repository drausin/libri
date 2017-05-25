package cmd

import (
	"fmt"
	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/author/keychain"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/librarian/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	librariansFlag = "librarians"
	envVarPrefix   = "LIBRI"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "test a set of librarian servers",
	Long:  `TODO (drausin) add longer description here`,
}

func init() {
	RootCmd.AddCommand(testCmd)

	testCmd.PersistentFlags().StringSliceP(librariansFlag, "a", nil,
		"comma-separated addresses (IPv4:Port) of librarian(s)")

	// bind viper flags
	viper.SetEnvPrefix(envVarPrefix) // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()             // read in environment variables that match
	if err := viper.BindPFlags(testCmd.PersistentFlags()); err != nil {
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

func getAuthor() (*author.Author, *zap.Logger, error) {
	config, logger, err := getAuthorConfig()
	if err != nil {
		return nil, logger, err
	}
	// since we're just doing tests, no need to worry about saving encrypted keychains and
	// generating more than one key on each
	authorKeys, selfReaderKeys := keychain.New(1), keychain.New(1)
	a, err := author.NewAuthor(config, authorKeys, selfReaderKeys, logger)
	return a, logger, err
}

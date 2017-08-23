package cmd

import (
	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	testLibrariansFlag = "testLibrarians"
)

// testCmd represents the test command
var testCmd = &cobra.Command{
	Use:   "test",
	Short: "test a set of librarian servers",
	Long:  `TODO (drausin) add longer description here`,
}

func init() {
	RootCmd.AddCommand(testCmd)

	testCmd.PersistentFlags().StringSliceP(testLibrariansFlag, "a", nil,
		"comma-separated addresses (IPv4:Port) of librarian(s)")
	testCmd.PersistentFlags().Int(timeoutFlag, 5,
		"timeout (seconds) for requests to librarians")

	// bind viper flags
	viper.SetEnvPrefix(envVarPrefix) // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()             // read in environment variables that match
	errors.MaybePanic(viper.BindPFlags(testCmd.PersistentFlags()))
}

type testAuthorGetter interface {
	get() (*author.Author, *zap.Logger, error)
}

type testAuthorGetterImpl struct {
	acg            authorConfigGetter
	librariansFlag string
}

func newTestAuthorGetter() testAuthorGetter {
	return &testAuthorGetterImpl{
		acg:            &authorConfigGetterImpl{},
		librariansFlag: testLibrariansFlag,
	}
}

func (g *testAuthorGetterImpl) get() (*author.Author, *zap.Logger, error) {
	config, logger, err := g.acg.get(g.librariansFlag)
	if err != nil {
		return nil, logger, err
	}
	// since we're just doing tests, no need to worry about saving encrypted keychains and
	// generating more than one key on each
	authorKeys, selfReaderKeys := keychain.New(1), keychain.New(1)
	a, err := author.NewAuthor(config, authorKeys, selfReaderKeys, logger)
	return a, logger, err
}

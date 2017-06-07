package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/librarian/server"
	lauthor "github.com/drausin/libri/libri/author"
	"go.uber.org/zap"
	"fmt"
	"github.com/drausin/libri/libri/author/keychain"
	"golang.org/x/crypto/ssh/terminal"
	"github.com/drausin/libri/libri/common/id"
	"io"
)

const (
	parallelismFlag = "parallelism"
	keychainDirFlag = "keychainsDir"
	passphraseVar = "passphrase"
)

// authorCmd represents the author command
var authorCmd = &cobra.Command{
	Use:   "author",
	Short: "run an author client of the libri network",
	Long: `TODO (drausin) add longer description and examples here`,
}

func init() {
	RootCmd.AddCommand(authorCmd)

	authorCmd.PersistentFlags().StringP(keychainDirFlag, "k", "", "local keychains directory")

	// bind viper flags
	viper.SetEnvPrefix(envVarPrefix) // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()             // read in environment variables that match
	if err := viper.BindPFlags(authorCmd.PersistentFlags()); err != nil {
		panic(err)
	}
}

type authorGetter interface {
	get(authorKeys, selfReaderKeys keychain.Keychain) (*author.Author, *zap.Logger, error)
}

type authorGetterImpl struct {
	acg authorConfigGetter
}

func newAuthorGetter() authorGetter {
	return &authorGetterImpl{&authorConfigGetterImpl{}}
}

func (g *authorGetterImpl) get(authorKeys, selfReaderKeys keychain.Keychain) (
	*author.Author, *zap.Logger, error) {

	config, logger, err := g.acg.get()
	if err != nil {
		return nil, nil, err
	}
	a, err := author.NewAuthor(config, authorKeys, selfReaderKeys, logger)
	return a, logger, err
}

type authorConfigGetter interface {
	get() (*author.Config, *zap.Logger, error)
}

type authorConfigGetterImpl struct {}

func (*authorConfigGetterImpl) get() (*author.Config, *zap.Logger, error) {
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

	logger.Info("author configuration",
		zap.String(librariansFlag, fmt.Sprintf("%v", config.LibrarianAddrs)),
		zap.String(dataDirFlag, config.DataDir),
		zap.Stringer(logLevelFlag, config.LogLevel),
	)
	return config, logger, nil
}

type passphraseGetter interface {
	get() (string, error)
}

type terminalPassphraseGetter struct {}

func (*terminalPassphraseGetter) get() (string, error) {
	passphraseBytes, err := terminal.ReadPassword(0)
	return string(passphraseBytes), err
}

// authorUploader just wraps an *author.Author Upload call that is hard to mock b/c *author.Author
// is a struct rather than an interface
type authorUploader interface {
	upload(author *lauthor.Author, content io.Reader, mediaType string) (id.ID, error)
}

type authorUploaderImpl struct {}

func (*authorUploaderImpl) upload(author *lauthor.Author, content io.Reader, mediaType string) (
	id.ID, error) {
	_, envelopeKey, err := author.Upload(content, mediaType)
	return envelopeKey, err
}

// authorDownloader just wraps an *author.Author Download call for the same reason as authorUploader
type authorDownloader interface {
	download(author *lauthor.Author, content io.Writer, envelopeKey id.ID) error
}

type authorDownloaderImpl struct {}

func (*authorDownloaderImpl) download(
	author *lauthor.Author, content io.Writer, envelopeKey id.ID,
) error {
	return author.Download(content, envelopeKey)
}

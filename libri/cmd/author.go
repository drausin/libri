package cmd

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/drausin/libri/libri/author"
	lauthor "github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/author/keychain"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/common/parse"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	parallelismFlag      = "parallelism"
	keychainDirFlag      = "keychainsDir"
	passphraseVar        = "passphrase"
	authorLibrariansFlag = "authorLibrarians"
	timeoutFlag          = "timeout"
)

// authorCmd represents the author command
var authorCmd = &cobra.Command{
	Use:   "author",
	Short: "operate an author client of the Libri network",
}

func init() {
	RootCmd.AddCommand(authorCmd)

	authorCmd.PersistentFlags().StringP(keychainDirFlag, "k", "", "local keychains directory")
	authorCmd.PersistentFlags().StringSliceP(authorLibrariansFlag, "a", nil,
		"comma-separated addresses (IPv4:Port) of librarian(s)")
	authorCmd.PersistentFlags().Int(timeoutFlag, 5,
		"timeout (seconds) for requests to librarians")

	// bind viper flags
	viper.SetEnvPrefix(envVarPrefix) // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()             // read in environment variables that match
	errors.MaybePanic(viper.BindPFlags(authorCmd.PersistentFlags()))
}

type authorGetter interface {
	get(authorKeys, selfReaderKeys keychain.GetterSampler) (*author.Author, *zap.Logger, error)
}

type authorGetterImpl struct {
	acg            authorConfigGetter
	librariansFlag string
}

func newAuthorGetter() authorGetter {
	return &authorGetterImpl{
		acg:            &authorConfigGetterImpl{},
		librariansFlag: authorLibrariansFlag,
	}
}

func (g *authorGetterImpl) get(authorKeys, selfReaderKeys keychain.GetterSampler) (
	*author.Author, *zap.Logger, error) {

	config, logger, err := g.acg.get(g.librariansFlag)
	if err != nil {
		return nil, nil, err
	}
	a, err := author.NewAuthor(config, authorKeys, selfReaderKeys, logger)
	return a, logger, err
}

type authorConfigGetter interface {
	get(librariansFlag string) (*author.Config, *zap.Logger, error)
}

type authorConfigGetterImpl struct{}

func (*authorConfigGetterImpl) get(librariansFlag string) (*author.Config, *zap.Logger, error) {
	config := author.NewDefaultConfig().
		WithDataDir(viper.GetString(dataDirFlag)).
		WithDefaultDBDir(). // depends on DataDir
		WithLogLevel(getLogLevel())
	timeout := time.Duration(viper.GetInt(timeoutFlag) * 1e9)
	config.Publish.PutTimeout = timeout
	config.Publish.GetTimeout = timeout

	logger := clogging.NewDevLogger(config.LogLevel)
	librarianNetAddrs, err := parse.Addrs(viper.GetStringSlice(librariansFlag))
	if err != nil {
		logger.Error("unable to parse librarian address", zap.Error(err))
		return nil, logger, err
	}
	config.WithLibrarianAddrs(librarianNetAddrs)

	WriteAuthorBanner(os.Stdout)
	logger.Info("author configuration",
		zap.String(librariansFlag, fmt.Sprintf("%v", config.LibrarianAddrs)),
		zap.String(dataDirFlag, config.DataDir),
		zap.Stringer(logLevelFlag, config.LogLevel),
		zap.Int(timeoutFlag, int(timeout.Seconds())),
	)
	return config, logger, nil
}

type passphraseGetter interface {
	get() (string, error)
}

type terminalPassphraseGetter struct{}

func (*terminalPassphraseGetter) get() (string, error) {
	passphraseBytes, err := terminal.ReadPassword(0)
	return string(passphraseBytes), err
}

// authorUploader just wraps an *author.Author Upload call that is hard to mock b/c *author.Author
// is a struct rather than an interface
type authorUploader interface {
	upload(author *lauthor.Author, content io.Reader, mediaType string) (id.ID, error)
}

type authorUploaderImpl struct{}

func (*authorUploaderImpl) upload(author *lauthor.Author, content io.Reader, mediaType string) (
	id.ID, error) {
	_, envelopeKey, err := author.Upload(content, mediaType)
	return envelopeKey, err
}

// authorDownloader just wraps an *author.Author Download call for the same reason as authorUploader
type authorDownloader interface {
	download(author *lauthor.Author, content io.Writer, envelopeKey id.ID) error
}

type authorDownloaderImpl struct{}

func (*authorDownloaderImpl) download(
	author *lauthor.Author, content io.Writer, envelopeKey id.ID,
) error {
	return author.Download(content, envelopeKey)
}

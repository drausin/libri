package cmd

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"

	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/author/io/common"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	maxContentSize = 12 * 1024 * 1024 // bytes
	minContentSize = 32               // bytes
)

const (
	nEntriesFlag = "nEntries"
)

// ioCmd represents the io command
var ioCmd = &cobra.Command{
	Use:   "io",
	Short: "check ability to upload and download entries",
	Long:  `TODO(drausin)`,
	Run: func(cmd *cobra.Command, args []string) {
		author, logger, err := newTestAuthorGetter().get()
		if err != nil {
			logger.Error("error while initializing author", zap.Error(err))
			os.Exit(1)
		}
		if err := newIOTester().test(author, logger); err != nil {
			logger.Error("error while testing io", zap.Error(err))
			os.Exit(1)
		}
	},
}

func init() {
	testCmd.AddCommand(ioCmd)

	ioCmd.Flags().IntP(nEntriesFlag, "n", 8, "number of entries")

	// bind viper flags
	viper.SetEnvPrefix("LIBRI") // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()        // read in environment variables that match
	if err := viper.BindPFlags(ioCmd.Flags()); err != nil {
		panic(err)
	}
}

type ioTester interface {
	test(author *author.Author, logger *zap.Logger) error
}

func newIOTester() ioTester {
	return &ioTesterImpl{
		au: &authorUploaderImpl{},
		ad: &authorDownloaderImpl{},
	}
}

type ioTesterImpl struct {
	au authorUploader
	ad authorDownloader
}

func (t *ioTesterImpl) test(author *author.Author, logger *zap.Logger) error {
	rng := rand.New(rand.NewSource(0))
	nEntries := viper.GetInt(nEntriesFlag)
	for i := 0; i < nEntries; i++ {
		nContentBytes := minContentSize +
			int(rng.Int31n(int32(maxContentSize-minContentSize)))
		contents := common.NewCompressableBytes(rng, nContentBytes).Bytes()
		mediaType := "application/x-pdf"
		if rng.Int()%2 == 0 {
			mediaType = "application/x-gzip"
		}

		uploadedBuf := bytes.NewReader(contents)
		envelopeKey, err := t.au.upload(author, uploadedBuf, mediaType)
		if err != nil {
			return err
		}
		downloadedBuf := new(bytes.Buffer)
		if err := t.ad.download(author, downloadedBuf, envelopeKey); err != nil {
			return err
		}
		downloaded := downloadedBuf.Bytes()
		if !bytes.Equal(contents, downloaded) {
			return fmt.Errorf(
				"uploaded content (%d bytes) does not equal downloaded (%d bytes)",
				len(contents), len(downloaded),
			)
		}
	}

	logger.Info("successfully uploaded & downloaded all entries",
		zap.Int("n_entries", nEntries),
	)
	return nil
}

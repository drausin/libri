package cmd

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
	"github.com/drausin/libri/libri/author"
	"bytes"
	"github.com/drausin/libri/libri/author/io/common"
	"math/rand"
	"fmt"
	"github.com/spf13/viper"
)

const (
	maxContentSize = 12 * 1024 * 1024	// bytes
	minContentSize = 32			// bytes
)

const (
	nEntriesFlag = "nEntries"
)

// ioCmd represents the io command
var ioCmd = &cobra.Command{
	Use:   "io",
	Short: "check ability to upload and download entries",
	Long: `TODO(drausin)`,
	Run: func(cmd *cobra.Command, args []string) {
		author, logger, err := newTestAuthorGetter().get()
		if err != nil {
			logger.Error("error while initializing author", zap.Error(err))
			os.Exit(1)
		}
		if err := testIO(author, logger); err != nil {
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

func testIO(author *author.Author, logger *zap.Logger) error {
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
		_, envelopeKey, err := author.Upload(uploadedBuf, mediaType)
		if err != nil {
			return err
		}
		downloadedBuf := new(bytes.Buffer)
		if err := author.Download(downloadedBuf, envelopeKey); err != nil {
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

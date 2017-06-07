package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"go.uber.org/zap"
	"github.com/pkg/errors"
	"github.com/drausin/libri/libri/common/id"
)

const (
	envelopeKeyFlag = "envelopeKey"
)

var (
	errMissingEnvelopeKey = errors.New("missing envelope key")
)

// downloadCmd represents the download command
var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "A brief description of your command",
	Long: `TODO (drausin) add long description and examples`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Work your own magic here
		fmt.Println("download called")
	},
}

func init() {
	authorCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().StringSliceP(librariansFlag, "a", nil,
		"comma-separated addresses (IPv4:Port) of librarian(s)")
	downloadCmd.Flags().Uint32P(parallelismFlag, "n", 3,
		"number of parallel processes")
	downloadCmd.Flags().StringP(filepathFlag, "f", "",
		"path of local file to write downloaded contents to")
	downloadCmd.Flags().StringP(envelopeKeyFlag, "e", "",
		"key of envelope to download")

	// bind viper flags
	viper.SetEnvPrefix(envVarPrefix) // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()             // read in environment variables that match
	if err := viper.BindPFlags(downloadCmd.Flags()); err != nil {
		panic(err)
	}
}

type fileDownloader interface {
	download() error
}

func newFileDownloader() fileDownloader {
	return &fileDownloaderImpl{
		ag:  newAuthorGetter(),
		ad: &authorDownloaderImpl{},
		kc: &keychainsGetterImpl{
			pg: &terminalPassphraseGetter{},
		},
	}
}

type fileDownloaderImpl struct {
	ag  authorGetter
	ad  authorDownloader
	kc  keychainsGetter
}

func (d *fileDownloaderImpl) download() error {
	envelopeKeyStr := viper.GetString(envelopeKeyFlag)
	if envelopeKeyStr == "" {
		return errMissingEnvelopeKey
	}
	envelopeKey, err := id.FromString(envelopeKeyStr)
	if err != nil {
		return err
	}
	downFilepath := viper.GetString(filepathFlag)
	if downFilepath == "" {
		return errMissingFilepath
	}
	authorKeys, selfReaderKeys, err := d.kc.get()
	if err != nil {
		return err
	}
	author, logger, err := d.ag.get(authorKeys, selfReaderKeys)
	if err != nil {
		return err
	}
	file, err := os.Open(downFilepath)
	if err != nil {
		return err
	}
	defer maybePanic(file.Close())
	logger.Info("downloading document",
		zap.Stringer("envelope_key", envelopeKey),
		zap.String("filepath", downFilepath),
	)
	return d.ad.download(author, file, envelopeKey)
}
package cmd

import (
	"os"

	cerrors "github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	envelopeKeyFlag  = "envelopeKey"
	downFilepathFlag = "downFilepath"
)

var (
	errMissingEnvelopeKey = errors.New("missing envelope key")
)

// downloadCmd represents the download command
var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "A brief description of your command",
	Long:  `TODO (drausin) add long description and examples`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return newFileDownloader().download()
	},
}

func init() {
	authorCmd.AddCommand(downloadCmd)

	downloadCmd.Flags().Uint32P(parallelismFlag, "n", 3,
		"number of parallel processes")
	downloadCmd.Flags().StringP(downFilepathFlag, "f", "",
		"path of local file to write downloaded contents to")
	downloadCmd.Flags().StringP(envelopeKeyFlag, "e", "",
		"key of envelope to download")

	// bind viper flags
	viper.SetEnvPrefix(envVarPrefix) // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()             // read in environment variables that match
	cerrors.MaybePanic(viper.BindPFlags(downloadCmd.Flags()))
}

type fileDownloader interface {
	download() error
}

func newFileDownloader() fileDownloader {
	return &fileDownloaderImpl{
		ag: newAuthorGetter(),
		ad: &authorDownloaderImpl{},
		kc: &keychainsGetterImpl{
			pg: &terminalPassphraseGetter{},
		},
	}
}

type fileDownloaderImpl struct {
	ag authorGetter
	ad authorDownloader
	kc keychainsGetter
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
	downFilepath := viper.GetString(downFilepathFlag)
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
	file, err := os.Create(downFilepath)
	if err != nil {
		return err
	}
	logger.Info("downloading document",
		zap.Stringer("envelope_key", envelopeKey),
		zap.String("filepath", downFilepath),
	)
	err = d.ad.download(author, file, envelopeKey)
	if err != nil {
		return err
	}
	return file.Close()
}

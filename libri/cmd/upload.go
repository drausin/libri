package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	lauthor "github.com/drausin/libri/libri/author"
	"github.com/pkg/errors"
	"github.com/drausin/libri/libri/author/keychain"
	"go.uber.org/zap"
	"os"
	"net/http"
	"io"
	"path/filepath"
	"mime"
)

const (
	filepathFlag = "filepath"
	octetMediaType = "application/octet-stream"
)

var (
	errKeychainsNotExist = errors.New("no keychains exist in the keychain directory")
	errMissingFilepath = errors.New("missing filepath")
)


// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "upload a local file to the libri network",
	Long: `TODO (drausin) add longer description and examples here`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := newFileUploader().upload(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	authorCmd.AddCommand(uploadCmd)

	uploadCmd.Flags().StringSliceP(librariansFlag, "a", nil,
		"comma-separated addresses (IPv4:Port) of librarian(s)")
	uploadCmd.Flags().Uint32P(parallelismFlag, "n", 3,
		"number of parallel processes")
	uploadCmd.Flags().StringP(filepathFlag, "f", "",
		"path of local file to upload")

	// bind viper flags
	viper.SetEnvPrefix(envVarPrefix) // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()             // read in environment variables that match
	if err := viper.BindPFlags(uploadCmd.Flags()); err != nil {
		panic(err)
	}
}

type fileUploader interface {
	upload() error
}

type fileUploaderImpl struct {
	ag authorGetter
	mtg mediaTypeGetter
	kc keychainsGetter
}

func newFileUploader() fileUploader {
	return &fileUploaderImpl{
		ag: newAuthorGetter(),
		mtg: &mediaTypeGetterImpl{},
		kc: &keychainsGetterImpl{
			pg: &terminalPassphraseGetter{},
		},
	}
}

func (u *fileUploaderImpl) upload() error {
	upFilepath := viper.GetString(filepathFlag)
	if upFilepath == "" {
		return errMissingFilepath
	}
	mediaType, err := u.mtg.get(upFilepath)
	file, err := os.Open(upFilepath)
	if err != nil {
		return err
	}
	authorKeys, selfReaderKeys, err := u.kc.get()
	if err != nil {
		return err
	}
	author, logger, err := u.ag.get(authorKeys, selfReaderKeys)

	logger.Info("uploading document",
		zap.String("filepath", upFilepath),
		zap.String("media_type", mediaType),
	)
	_, _, err = author.Upload(file, mediaType)
	return err
}

type mediaTypeGetter interface {
	get(upFilepath string) (string, error)
}

type mediaTypeGetterImpl struct {}

func (*mediaTypeGetterImpl) get(upFilepath string) (string, error) {
	file, err := os.Open(upFilepath)
	if err != nil {
		return "", err
	}
	head := make([]byte, 512)
	_, err = file.Read(head)
	file.Close()
	if err != nil && err != io.EOF {
		return "", err
	}
	mediaType := http.DetectContentType(head)
	if mediaType != octetMediaType {
		// sniffing head of file worked
		return mediaType, nil
	}
	mediaType = mime.TypeByExtension(filepath.Ext(upFilepath))
	if mediaType != "" {
		// get by extension worked
		return mediaType, nil
	}

	// fallback
	return octetMediaType, nil
}

type keychainsGetter interface {
	get() (keychain.Keychain, keychain.Keychain, error)
}

type keychainsGetterImpl struct {
	pg passphraseGetter
}

func (g *keychainsGetterImpl) get() (keychain.Keychain, keychain.Keychain, error) {
	keychainDir := viper.GetString(keychainDirFlag)
	if keychainDir == "" {
		return nil, nil, errMissingKeychainDir
	}
	missing, err := lauthor.MissingKeychains(keychainDir)
	if err != nil {
		return nil, nil, err
	}
	if missing {
		return nil, nil, errKeychainsNotExist
	}
	passphrase := viper.GetString(passphraseVar) // intentionally not bound to flag
	if passphrase == "" {
		// get passphrase from terminal
		passphrase, err = g.pg.get()
		if err != nil {
			return nil, nil, err
		}
	}
	return lauthor.LoadKeychains(keychainDir, passphrase)
}


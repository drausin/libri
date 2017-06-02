package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
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
		if err := uploadFile(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	authorCmd.AddCommand(uploadCmd)

	testCmd.Flags().StringSliceP(librariansFlag, "a", nil,
		"comma-separated addresses (IPv4:Port) of librarian(s)")
	testCmd.Flags().Uint32P(parallelismFlag, "n", 3,
		"number of parallel processes")
	uploadCmd.Flags().StringP(filepathFlag, "f", "",
		"path of local file to upload")

	// bind viper flags
	viper.SetEnvPrefix(envVarPrefix) // look for env vars with "LIBRI_" prefix
	viper.AutomaticEnv()             // read in environment variables that match
	if err := viper.BindPFlags(testCmd.PersistentFlags()); err != nil {
		panic(err)
	}
}

func uploadFile() error {
	upFilepath := viper.GetString(filepathFlag)
	if upFilepath == "" {
		return errMissingFilepath
	}
	mediaType, err := getMediaType(upFilepath)
	file, err := os.Open(upFilepath)
	if err != nil {
		return err
	}
	authorKeys, selfReaderKeys, err := getKeychains()
	if err != nil {
		return err
	}
	author, logger, err := getAuthor(authorKeys, selfReaderKeys)

	logger.Info("uploading document",
		zap.String("filepath", upFilepath),
		zap.String("media_type", mediaType),
	)
	_, _, err = author.Upload(file, mediaType)
	return err
}

func getMediaType(upFilepath string) (string, error) {
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
	mediaType1 := http.DetectContentType(head)
	if mediaType1 != octetMediaType {
		// sniffing head of file worked
		return mediaType1, nil
	}
	mediaType2 := mime.TypeByExtension(filepath.Ext(upFilepath))
	if mediaType2 != "" {
		// sniffing extension worked
		return mediaType2, nil
	}

	// fallback
	return octetMediaType, nil
}

func getKeychains() (keychain.Keychain, keychain.Keychain, error) {
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
	passphrase, err := getPassphrase()
	if err != nil {
		return nil, nil, err
	}
	return lauthor.LoadKeychains(keychainDir, passphrase)
}

func getPassphrase() (string, error) {
	passphrase := viper.GetString(passphraseVar) // intentionally not bound to flag
	if passphrase != "" {
		return passphrase, nil
	}
	fmt.Print("Enter keychains passphrase: ")
	passphraseBytes, err := terminal.ReadPassword(0)
	if err != nil {
		return "", err
	}
	return string(passphraseBytes), nil
}

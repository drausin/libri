package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/author/keychain"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/crypto/ssh/terminal"
	"os"
)

var (
	errMismatchedPassphrase    = errors.New("second passphrase does not match first")
	errConfirmationNotRecorded = errors.New(`confirmation is not "RECORDED"`)
	errMissingKeychainDir      = errors.New("keychainDir cannot be empty")
	errKeychainsExist          = errors.New("keychains already exist in keychain directory")
	recordedInput              = "RECORDED"
)

// initCmd represents the init command
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initialize author keychains",
	Long:  `TODO (drausin) add longer description and examples here`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := createKeychains(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	authorCmd.AddCommand(initCmd)
}

func createKeychains() error {
	keychainDir := viper.GetString(keychainDirFlag)
	if keychainDir == "" {
		return errMissingKeychainDir
	}
	missing, err := author.MissingKeychains(keychainDir)
	if err != nil {
		return err
	}
	if !missing {
		return errKeychainsExist
	}
	passphrase, err := setPassphrase()
	if err != nil {
		return err
	}

	logger := clogging.NewDevLogger(getLogLevel())
	logger.Info("creating keychains")
	return author.CreateKeychains(logger, keychainDir, passphrase,
		keychain.LightScryptN, keychain.LightScryptP)
}

func setPassphrase() (string, error) {
	passphrase := viper.GetString(passphraseVar) // intentionally not bound to flag
	if passphrase != "" {
		return passphrase, nil
	}

	fmt.Print("Enter passphrase for new keychains: ")
	passphraseBytes, err := terminal.ReadPassword(0)
	if err != nil {
		return "", err
	}
	fmt.Printf("\nEnter passphrase again: ")
	repeatedBytes, err := terminal.ReadPassword(0)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(passphraseBytes, repeatedBytes) {
		return "", errMismatchedPassphrase
	}
	fmt.Println("\n\nRecord your passphrase somewhere safe. You won't be able to recover it!")
	fmt.Println("Safe places include:")
	fmt.Println(" - password manager app (e.g., 1Password, LastPass)")
	fmt.Println(" - physically written down somewhere safe")
	fmt.Println()
	fmt.Print(`Enter "RECORDED" once you have done this: `)
	reader := bufio.NewReader(os.Stdin)
	confirmation, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	confirmation = confirmation[:len(confirmation)-1] // drop \n at end
	if confirmation != recordedInput {
		fmt.Println(confirmation + "|" + recordedInput)
		return "", errConfirmationNotRecorded
	}
	return string(passphraseBytes), nil
}

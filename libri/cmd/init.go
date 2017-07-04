package cmd

import (
	"bufio"
	"fmt"
	"os"

	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/author/keychain"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
		if err := newKeychainCreator().create(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	authorCmd.AddCommand(initCmd)
}

type keychainCreator interface {
	create() error
}

func newKeychainCreator() keychainCreator {
	return &keychainCreatorImpl{
		ps: &passphraseSetterImpl{
			pg1:    &terminalPassphraseGetter{},
			pg2:    &terminalPassphraseGetter{},
			reader: bufio.NewReader(os.Stdin),
		},
		scryptN: keychain.LightScryptN,
		scryptP: keychain.LightScryptP,
	}
}

type keychainCreatorImpl struct {
	ps      passphraseSetter
	scryptN int
	scryptP int
}

func (c *keychainCreatorImpl) create() error {
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
	passphrase, err := c.ps.set()
	if err != nil {
		return err
	}

	logger := clogging.NewDevLogger(getLogLevel())
	logger.Info("creating keychains")
	return author.CreateKeychains(logger, keychainDir, passphrase, c.scryptN, c.scryptP)
}

type passphraseSetter interface {
	set() (string, error)
}

type passphraseSetterImpl struct {
	pg1    passphraseGetter
	pg2    passphraseGetter
	reader *bufio.Reader
}

func (s *passphraseSetterImpl) set() (string, error) {
	passphrase := viper.GetString(passphraseVar) // intentionally not bound to flag for a tad
	if passphrase != "" {
		return passphrase, nil
	}

	fmt.Print("Enter passphrase for new keychains: ")
	passphrase, err := s.pg1.get()
	if err != nil {
		return "", err
	}
	fmt.Printf("\nEnter passphrase again: ")
	repeated, err := s.pg2.get()
	if err != nil {
		return "", err
	}
	if passphrase != repeated {
		return "", errMismatchedPassphrase
	}
	fmt.Println("\n\nRecord your passphrase somewhere safe. You won't be able to recover it!")
	fmt.Println("Safe places include:")
	fmt.Println(" - password manager app (e.g., 1Password, LastPass)")
	fmt.Println(" - physically written down somewhere safe")
	fmt.Println()
	fmt.Print(`Enter "RECORDED" once you have done this: `)
	confirmation, err := s.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	confirmation = confirmation[:len(confirmation)-1] // drop \n at end
	if confirmation != recordedInput {
		return "", errConfirmationNotRecorded
	}
	return passphrase, nil
}

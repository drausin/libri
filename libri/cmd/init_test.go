package cmd

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/common/logging"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestInitCmd_err(t *testing.T) {
	dataDir, err := ioutil.TempDir("", "test-author-data-dir")
	defer func() { err = os.RemoveAll(dataDir) }()
	viper.Set(dataDirFlag, dataDir)

	viper.Set(keychainDirFlag, "")
	err = initCmd.RunE(initCmd, []string{})
	assert.NotNil(t, err)
}

func TestKeychainCreator_create_ok(t *testing.T) {
	keychainDir, err := ioutil.TempDir("", "test-keychains")
	defer func() { err = os.RemoveAll(keychainDir) }()
	assert.Nil(t, err)
	passphrase := "some test passphrase"
	viper.Set(keychainDirFlag, keychainDir)

	kc := keychainCreatorImpl{
		ps:      &fixedPassphraseSetter{passphrase: passphrase},
		scryptN: veryLightScryptN,
		scryptP: veryLightScryptP,
	}
	err = kc.create()
	assert.Nil(t, err)
}

func TestKeychainCreator_create_err(t *testing.T) {
	keychainDir, err := ioutil.TempDir("", "test-keychains")
	defer func() { err = os.RemoveAll(keychainDir) }()
	logger := logging.NewDevInfoLogger()
	setPassphrase := "some passphrase"
	assert.Nil(t, err)

	// should error when keychain dir isn't set
	viper.Set(keychainDirFlag, "")
	kc1 := &keychainCreatorImpl{}
	err = kc1.create()
	assert.Equal(t, errMissingKeychainDir, err)

	// MissingKeychains error should bubble up
	viper.Set(keychainDirFlag, keychainDir)
	keychainFilepath := path.Join(keychainDir, author.AuthorKeychainFilename)
	err = author.CreateKeychain(logger, keychainFilepath, setPassphrase,
		veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)
	kc2 := &keychainCreatorImpl{}
	err = kc2.create()
	assert.NotNil(t, err)

	// should throw error when keychains exist
	keychainFilepath = path.Join(keychainDir, author.SelfReaderKeychainFilename)
	err = author.CreateKeychain(logger, keychainFilepath, setPassphrase,
		veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)
	kc3 := &keychainCreatorImpl{}
	err = kc3.create()
	assert.NotNil(t, err)

	err = os.RemoveAll(keychainDir)
	assert.Nil(t, err)

	// passphrase setter error should bubble up
	kc4 := &keychainCreatorImpl{
		ps: &fixedPassphraseSetter{err: errors.New("some set error")},
	}
	err = kc4.create()
	assert.NotNil(t, err)

	kc5 := keychainCreatorImpl{
		ps:      &fixedPassphraseSetter{passphrase: setPassphrase},
		scryptN: -1, // will cause error
		scryptP: -1,
	}
	err = kc5.create()
	assert.NotNil(t, err)
}

type fixedPassphraseSetter struct {
	passphrase string
	err        error
}

func (s *fixedPassphraseSetter) set() (string, error) {
	return s.passphrase, s.err
}

func TestPassphraseSetter_set_ok(t *testing.T) {
	setPassphrase := "some passphrase"
	viper.Set(passphraseVar, setPassphrase)

	// check can get passphrase from viper
	ps1 := &passphraseSetterImpl{}
	pass1, err := ps1.set()
	assert.Nil(t, err)
	assert.Equal(t, setPassphrase, pass1)

	// check can get passphrase from input
	viper.Set(passphraseVar, "")
	ps2 := &passphraseSetterImpl{
		pg1:    &fixedPassphraseGetter{passphrase: setPassphrase},
		pg2:    &fixedPassphraseGetter{passphrase: setPassphrase},
		reader: bufio.NewReader(bytes.NewBuffer([]byte(recordedInput + "\n"))),
	}

	pass2, err := ps2.set()
	assert.Nil(t, err)
	assert.Equal(t, setPassphrase, pass2)
}

func TestPassphraseSetter_set_err(t *testing.T) {
	setPassphrase := "some passphrase"
	viper.Set(passphraseVar, "")

	ps1 := &passphraseSetterImpl{
		pg1: &fixedPassphraseGetter{err: errors.New("some get error")},
	}
	pass1, err := ps1.set()
	assert.NotNil(t, err)
	assert.Zero(t, pass1)

	ps2 := &passphraseSetterImpl{
		pg1: &fixedPassphraseGetter{passphrase: setPassphrase},
		pg2: &fixedPassphraseGetter{err: errors.New("some get error")},
	}
	pass2, err := ps2.set()
	assert.NotNil(t, err)
	assert.Zero(t, pass2)

	ps3 := &passphraseSetterImpl{
		pg1: &fixedPassphraseGetter{passphrase: setPassphrase},
		pg2: &fixedPassphraseGetter{passphrase: "something different"},
	}
	pass3, err := ps3.set()
	assert.Equal(t, errMismatchedPassphrase, err)
	assert.Zero(t, pass3)

	ps4 := &passphraseSetterImpl{
		pg1:    &fixedPassphraseGetter{passphrase: setPassphrase},
		pg2:    &fixedPassphraseGetter{passphrase: setPassphrase},
		reader: bufio.NewReader(bytes.NewBuffer([]byte{})), // will yield io.EOF error
	}
	pass4, err := ps4.set()
	assert.NotNil(t, err)
	assert.Zero(t, pass4)

	ps5 := &passphraseSetterImpl{
		pg1:    &fixedPassphraseGetter{passphrase: setPassphrase},
		pg2:    &fixedPassphraseGetter{passphrase: setPassphrase},
		reader: bufio.NewReader(bytes.NewBuffer([]byte("NOT RECORDED\n"))),
	}
	pass5, err := ps5.set()
	assert.Equal(t, errConfirmationNotRecorded, err)
	assert.Zero(t, pass5)
}

package cmd

import (
	"testing"
	"github.com/drausin/libri/libri/author"
	"io/ioutil"
	"os"
	"github.com/stretchr/testify/assert"
	"github.com/drausin/libri/libri/common/logging"
	"github.com/spf13/viper"
	"path"
	"github.com/pkg/errors"
	"bytes"
	"compress/gzip"
)

const (
	veryLightScryptN = 2
	veryLightScryptP = 1
)

func TestMediaTypeGetter_get_ok(t *testing.T) {
	uncompressed := bytes.Repeat([]byte("these bytes are uncompressed"), 25)
	compressed := new(bytes.Buffer)
	w := gzip.NewWriter(compressed)
	_, err := w.Write(uncompressed)
	assert.Nil(t, err)
	mtg := mediaTypeGetterImpl{}

	// should infer media type from header bytes of file
	compressedFile, err := ioutil.TempFile("", "compressed-file")
	assert.Nil(t, err)
	err = compressedFile.Close()
	defer maybePanic(os.Remove(compressedFile.Name()))
	assert.Nil(t, err)
	err = ioutil.WriteFile(compressedFile.Name(), compressed.Bytes(), 0666)
	assert.Nil(t, err)

	mediaType, err := mtg.get(compressedFile.Name())
	assert.Nil(t, err)
	assert.Equal(t, "application/x-gzip", mediaType)

	// should infer media type from extension
	testDir, err := ioutil.TempDir("", "test")
	assert.Nil(t, err)
	emptyGzipFile := path.Join(testDir, "empty.pdf")
	f, err := os.Create(emptyGzipFile)
	assert.Nil(t, err)
	err = f.Close()
	assert.Nil(t, err)

	mediaType, err = mtg.get(emptyGzipFile)
	assert.Nil(t, err)
	assert.Equal(t, "application/pdf", mediaType)

	// should fall back to default
	emptyFileNoExt := path.Join(testDir, "empty")
	f, err = os.Create(emptyFileNoExt)
	assert.Nil(t, err)
	err = f.Close()
	assert.Nil(t, err)

	mediaType, err = mtg.get(emptyFileNoExt)
	assert.Nil(t, err)
	assert.Equal(t, octetMediaType, mediaType)

	maybePanic(os.RemoveAll(testDir))
}

func TestMediaTypeGetter_get_err(t *testing.T) {
	mtg := mediaTypeGetterImpl{}

	mediaType, err := mtg.get("/path/to/nonexistant/file")
	assert.NotNil(t, err)
	assert.Zero(t, mediaType)
}

func TestKeychainsGetter_get_ok(t *testing.T) {
	keychainDir, err := ioutil.TempDir("", "test-keychains")
	assert.Nil(t, err)
	logger := server.NewDevInfoLogger()
	passphrase := "some test passphrase"
	err = author.CreateKeychains(logger, keychainDir, passphrase,
		veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)
	viper.Set(keychainDirFlag, keychainDir)

	// check getting passphrase from (mocked) terminal
	kg1 := &keychainsGetterImpl{
		&fixedPassphraseGetter{passphrase: passphrase},
	}
	authorKeys, selfReaderKeys, err := kg1.get()
	assert.Nil(t, err)
	assert.NotNil(t, authorKeys)
	assert.NotNil(t, selfReaderKeys)

	// check getting passphrase from viper
	kg2 := &keychainsGetterImpl{}
	viper.Set(passphraseVar, passphrase)
	authorKeys, selfReaderKeys, err = kg2.get()
	assert.Nil(t, err)
	assert.NotNil(t, authorKeys)
	assert.NotNil(t, selfReaderKeys)

	err = os.RemoveAll(keychainDir)
	assert.Nil(t, err)
}

func TestKeychainsGetter_get_err(t *testing.T) {
	keychainDir, err := ioutil.TempDir("", "test-keychains")
	assert.Nil(t, err)
	logger := server.NewDevInfoLogger()
	passphrase := "some test passphrase"

	// should error on missing keychainDirFlag
	viper.Set(keychainDirFlag, "")
	kg1 := &keychainsGetterImpl{}
	authorKeys, selfReaderKeys, err := kg1.get()
	assert.Equal(t, errMissingKeychainDir, err)
	assert.Nil(t, authorKeys)
	assert.Nil(t, selfReaderKeys)

	// should error on one missing keychain
	keychainFilepath := path.Join(keychainDir, author.AuthorKeychainFilename)
	err = author.CreateKeychain(logger, keychainFilepath, passphrase,
		veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)
	kg2 := &keychainsGetterImpl{}
	authorKeys, selfReaderKeys, err = kg2.get()
	assert.NotNil(t, err)
	assert.Nil(t, authorKeys)
	assert.Nil(t, selfReaderKeys)
	err = os.Remove(keychainFilepath)
	assert.Nil(t, err)

	// should error on both missing keychains
	viper.Set(keychainDirFlag, keychainDir)
	kg3 := &keychainsGetterImpl{}
	authorKeys, selfReaderKeys, err = kg3.get()
	assert.Equal(t, errKeychainsNotExist, err)
	assert.Nil(t, authorKeys)
	assert.Nil(t, selfReaderKeys)

	// should error on passphrase error
	viper.Set(passphraseVar, "")
	kg4 := &keychainsGetterImpl{
		&fixedPassphraseGetter{err: errors.New("some passphrase error")},
	}
	authorKeys, selfReaderKeys, err = kg4.get()
	assert.NotNil(t, err)
	assert.Nil(t, authorKeys)
	assert.Nil(t, selfReaderKeys)

	err = os.RemoveAll(keychainDir)
	assert.Nil(t, err)
}

func maybePanic(err error) {
	if err != nil {
		panic(err)
	}
}

type fixedPassphraseGetter struct {
	passphrase string
	err error
}

func (f *fixedPassphraseGetter) get() (string, error) {
	return f.passphrase, f.err
}

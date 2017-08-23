package cmd

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/drausin/libri/libri/author"
	lauthor "github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/author/keychain"
	cerrors "github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/logging"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

const (
	veryLightScryptN = 2
	veryLightScryptP = 1
)

// TestUploadCmd_ok : this path is annoying to test b/c it involves lots of setup; but this is
// tested in acceptance/local-demo.sh, so it's ok to skip here

func TestUploadCmd_err(t *testing.T) {
	dataDir, err := ioutil.TempDir("", "test-author-data-dir")
	defer func() { err = os.RemoveAll(dataDir) }()
	viper.Set(dataDirFlag, dataDir)

	// check upload() error bubbles up
	viper.Set(upFilepathFlag, "")
	err = uploadCmd.RunE(uploadCmd, []string{})
	assert.Equal(t, errMissingFilepath, err)
}

func TestFileUploader_upload_ok(t *testing.T) {
	u := &fileUploaderImpl{
		ag: &fixedAuthorGetter{
			author: nil, // ok since we're passing it into a mocked method anyway
			logger: server.NewDevInfoLogger(),
		},
		au:  &fixedAuthorUploader{},
		mtg: &fixedMediaTypeGetter{}, // ok that mediaType is nil since passing to mock
		kc:  &fixedKeychainsGetter{}, // ok that KCs are null for same reason
	}
	toUploadFile, err := ioutil.TempFile("", "to-upload")
	defer func() { cerrors.MaybePanic(os.Remove(toUploadFile.Name())) }()
	assert.Nil(t, err)
	err = toUploadFile.Close()
	assert.Nil(t, err)
	viper.Set(upFilepathFlag, toUploadFile.Name())

	err = u.upload()
	assert.Nil(t, err)
}

func TestFileUploader_upload_err(t *testing.T) {

	// should error on missing filepath
	u1 := &fileUploaderImpl{}
	viper.Set(upFilepathFlag, "")
	err := u1.upload()
	assert.Equal(t, errMissingFilepath, err)

	// error getting media type should bubble up
	u2 := &fileUploaderImpl{
		mtg: &fixedMediaTypeGetter{err: errors.New("some get error")},
	}
	viper.Set(upFilepathFlag, "some/upload/filepath")
	err = u2.upload()
	assert.NotNil(t, err)

	// non-existent file should throw error
	u3 := &fileUploaderImpl{
		mtg: &fixedMediaTypeGetter{}, // ok that mediaType is nil since passing to mock
	}
	viper.Set(upFilepathFlag, "some/upload/filepath")
	err = u3.upload()
	assert.NotNil(t, err)

	// error getting author keys should bubble up
	toUploadFile, err := ioutil.TempFile("", "to-upload")
	defer func() { cerrors.MaybePanic(os.Remove(toUploadFile.Name())) }()
	assert.Nil(t, err)
	err = toUploadFile.Close()
	assert.Nil(t, err)
	viper.Set(upFilepathFlag, toUploadFile.Name())
	u4 := &fileUploaderImpl{
		mtg: &fixedMediaTypeGetter{}, // ok that mediaType is nil since passing to mock
		kc:  &fixedKeychainsGetter{err: errors.New("some get error")},
	}
	err = u4.upload()
	assert.NotNil(t, err)

	// error getting author should bubble up
	viper.Set(upFilepathFlag, toUploadFile.Name())
	u5 := &fileUploaderImpl{
		ag:  &fixedAuthorGetter{err: errors.New("some get error")},
		mtg: &fixedMediaTypeGetter{}, // ok that mediaType is nil since passing to mock
		kc:  &fixedKeychainsGetter{}, // ok that KCs are null for same reason
	}
	err = u5.upload()
	assert.NotNil(t, err)

	// upload error should bubble up
	viper.Set(upFilepathFlag, toUploadFile.Name())
	u6 := &fileUploaderImpl{
		ag: &fixedAuthorGetter{
			author: nil, // ok since we're passing it into a mocked method anyway
			logger: server.NewDevInfoLogger(),
		},
		au:  &fixedAuthorUploader{err: errors.New("some upload error")},
		mtg: &fixedMediaTypeGetter{}, // ok that mediaType is nil since passing to mock
		kc:  &fixedKeychainsGetter{}, // ok that KCs are null for same reason
	}
	err = u6.upload()
	assert.NotNil(t, err)
}

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
	defer func() { cerrors.MaybePanic(os.Remove(compressedFile.Name())) }()
	assert.Nil(t, err)
	err = ioutil.WriteFile(compressedFile.Name(), compressed.Bytes(), 0666)
	assert.Nil(t, err)

	mediaType, err := mtg.get(compressedFile.Name())
	assert.Nil(t, err)
	assert.Equal(t, "application/x-gzip", mediaType)

	// should infer media type from extension
	testDir, err := ioutil.TempDir("", "test")
	defer func() { cerrors.MaybePanic(os.RemoveAll(testDir)) }()
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
}

func TestMediaTypeGetter_get_err(t *testing.T) {
	mtg := mediaTypeGetterImpl{}

	mediaType, err := mtg.get("/path/to/nonexistant/file")
	assert.NotNil(t, err)
	assert.Zero(t, mediaType)
}

func TestKeychainsGetter_get_ok(t *testing.T) {
	keychainDir, err := ioutil.TempDir("", "test-keychains")
	defer func() { cerrors.MaybePanic(os.RemoveAll(keychainDir)) }()
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
}

func TestKeychainsGetter_get_err(t *testing.T) {
	keychainDir, err := ioutil.TempDir("", "test-keychains")
	defer func() { cerrors.MaybePanic(os.RemoveAll(keychainDir)) }()
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
}

type fixedAuthorUploader struct {
	envelopeKey id.ID
	err         error
}

func (f *fixedAuthorUploader) upload(author *lauthor.Author, content io.Reader, mediaType string) (
	id.ID, error) {
	return f.envelopeKey, f.err
}

type fixedAuthorGetter struct {
	author *lauthor.Author
	logger *zap.Logger
	err    error
}

func (f *fixedAuthorGetter) get(authorKeys, selfReaderKeys keychain.GetterSampler) (
	*lauthor.Author, *zap.Logger, error) {
	return f.author, f.logger, f.err
}

type fixedMediaTypeGetter struct {
	mediaType string
	err       error
}

func (f *fixedMediaTypeGetter) get(upFilepath string) (string, error) {
	return f.mediaType, f.err
}

type fixedKeychainsGetter struct {
	authorKeys     keychain.GetterSampler
	selfReaderKeys keychain.GetterSampler
	err            error
}

func (f *fixedKeychainsGetter) get() (keychain.GetterSampler, keychain.GetterSampler, error) {
	return f.authorKeys, f.selfReaderKeys, f.err
}

type fixedPassphraseGetter struct {
	passphrase string
	err        error
}

func (f *fixedPassphraseGetter) get() (string, error) {
	return f.passphrase, f.err
}

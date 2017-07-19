package cmd

import (
	"bytes"
	"io"
	"math/rand"
	"testing"
	lauthor "github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/logging"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"os"
)

func TestIOCmd_ok(t *testing.T) {
	dataDir, err := ioutil.TempDir("", "test-author-data-dir")
	defer func() { err = os.RemoveAll(dataDir) }()
	viper.Set(dataDirFlag, dataDir)

	viper.Set(nEntriesFlag, 0)
	viper.Set(testLibrariansFlag, "localhost:20200 localhost:20201")
	err = ioCmd.RunE(ioCmd, []string{})
	assert.Nil(t, err)
}

func TestIOCmd_err(t *testing.T) {
	dataDir, err := ioutil.TempDir("", "test-author-data-dir")
	defer func() { err = os.RemoveAll(dataDir) }()
	viper.Set(dataDirFlag, dataDir)

	// check newTestAuthorGetter() error bubbles up
	viper.Set(testLibrariansFlag, "bad librarians address")
	err = ioCmd.RunE(ioCmd, []string{})
	assert.NotNil(t, err)

	// check ioTest.test() error from ok but missing librarians bubbles up
	viper.Set(testLibrariansFlag, "localhost:20200 localhost:20201")
	viper.Set(nEntriesFlag, 1)
	err = ioCmd.RunE(ioCmd, []string{})
	assert.NotNil(t, err)
}

func TestIOTester_test_ok(t *testing.T) {
	viper.Set(nEntriesFlag, 1)
	aud := &fixedAuthorUploaderDownloader{
		rng:      rand.New(rand.NewSource(0)),
		uploaded: make(map[string]io.WriterTo),
	}
	iot := &ioTesterImpl{
		au: aud,
		ad: aud,
	}
	logger := server.NewDevInfoLogger()
	err := iot.test(nil, logger)
	assert.Nil(t, err)
}

func TestIOTester_test_err(t *testing.T) {
	viper.Set(nEntriesFlag, 1)
	logger := server.NewDevInfoLogger()

	iot1 := &ioTesterImpl{
		au: &fixedAuthorUploaderDownloader{uploadErr: errors.New("some upload err")},
	}
	err := iot1.test(nil, logger)
	assert.NotNil(t, err)

	iot2 := &ioTesterImpl{
		au: &fixedAuthorUploaderDownloader{
			rng:      rand.New(rand.NewSource(0)),
			uploaded: make(map[string]io.WriterTo),
		},
		ad: &fixedAuthorUploaderDownloader{downloadErr: errors.New("some download err")},
	}
	err = iot2.test(nil, logger)
	assert.NotNil(t, err)

	iot3 := &ioTesterImpl{
		au: &fixedAuthorUploaderDownloader{
			rng:      rand.New(rand.NewSource(0)),
			uploaded: make(map[string]io.WriterTo),
		},
		ad: &fixedAuthorUploaderDownloader{
			rng:      rand.New(rand.NewSource(0)),
			uploaded: make(map[string]io.WriterTo),
		},
	}
	err = iot3.test(nil, logger)
	assert.NotNil(t, err)
}

type fixedAuthorUploaderDownloader struct {
	rng         *rand.Rand
	uploaded    map[string]io.WriterTo
	uploadErr   error
	downloadErr error
}

func (f *fixedAuthorUploaderDownloader) upload(
	author *lauthor.Author, content io.Reader, mediaType string,
) (id.ID, error) {
	if f.uploadErr != nil {
		return nil, f.uploadErr
	}
	buf := new(bytes.Buffer)
	_, err := buf.ReadFrom(content)
	key := id.NewPseudoRandom(f.rng)
	f.uploaded[key.String()] = buf
	return key, err
}

func (f *fixedAuthorUploaderDownloader) download(
	author *lauthor.Author, content io.Writer, envelopeKey id.ID,
) error {
	if f.downloadErr != nil {
		return f.downloadErr
	}
	doc := f.uploaded[envelopeKey.String()]
	if doc == nil {
		return nil
	}
	_, err := doc.WriteTo(content)
	return err
}

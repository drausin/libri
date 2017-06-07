package cmd

import (
	"errors"
	lauthor "github.com/drausin/libri/libri/author"
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/common/logging"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

func TestNewFileDownloader(t *testing.T) {
	u := newFileDownloader()
	assert.NotNil(t, u)
}

func TestFileDownloader_download_ok(t *testing.T) {
	d := &fileDownloaderImpl{
		ag: &fixedAuthorGetter{
			author: nil, // ok since we're passing it into a mocked method anyway
			logger: server.NewDevInfoLogger(),
		},
		ad: &fixedAuthorDownloader{},
		kc: &fixedKeychainsGetter{}, // ok that KCs are null since passing to mock
	}
	toDownloadFile, err := ioutil.TempFile("", "to-download")
	assert.Nil(t, err)
	err = toDownloadFile.Close()
	assert.Nil(t, err)
	viper.Set(downFilepathFlag, toDownloadFile.Name())
	viper.Set(envelopeKeyFlag, id.LowerBound.String())

	err = d.download()
	assert.Nil(t, err)

	err = os.Remove(toDownloadFile.Name())
	assert.Nil(t, err)
}

func TestFileDownloader_download_err(t *testing.T) {
	// should error on missing envelopeKey
	d1 := &fileDownloaderImpl{}
	viper.Set(envelopeKeyFlag, "")
	err := d1.download()
	assert.Equal(t, errMissingEnvelopeKey, err)

	// should error on bad envelope key
	d2 := &fileDownloaderImpl{}
	viper.Set(envelopeKeyFlag, "0")
	err = d2.download()
	assert.NotNil(t, err)

	// should error on missing filepath
	d3 := &fileDownloaderImpl{}
	viper.Set(envelopeKeyFlag, id.LowerBound.String())
	viper.Set(downFilepathFlag, "")
	err = d3.download()
	assert.Equal(t, errMissingFilepath, err)

	// error getting author keys should bubble up
	toDownloadFile, err := ioutil.TempFile("", "to-download")  // TODO (drausin) change to tempDir
	assert.Nil(t, err)
	err = toDownloadFile.Close()
	viper.Set(downFilepathFlag, toDownloadFile.Name())
	d4 := &fileDownloaderImpl{
		kc: &fixedKeychainsGetter{err: errors.New("some get error")},
	}
	err = d4.download()
	assert.NotNil(t, err)

	// error getting author should bubble up
	viper.Set(downFilepathFlag, toDownloadFile.Name())
	d5 := &fileDownloaderImpl{
		ag: &fixedAuthorGetter{err: errors.New("some get error")},
		kc: &fixedKeychainsGetter{}, // ok that KCs are null since passing to mock
	}
	err = d5.download()
	assert.NotNil(t, err)

	// download error should bubble up
	viper.Set(downFilepathFlag, toDownloadFile.Name())
	d6 := &fileDownloaderImpl{
		ag: &fixedAuthorGetter{
			author: nil, // ok since we're passing it into a mocked method anyway
			logger: server.NewDevInfoLogger(),
		},
		ad: &fixedAuthorDownloader{err: errors.New("some download error")},
		kc: &fixedKeychainsGetter{}, // ok that KCs are null since passing to mock
	}
	err = d6.download()
	assert.NotNil(t, err)

	err = os.Remove(toDownloadFile.Name())
	assert.Nil(t, err)
}

type fixedAuthorDownloader struct {
	err error
}

func (f *fixedAuthorDownloader) download(
	author *lauthor.Author, content io.Writer, envelopeKey id.ID,
) error {
	return f.err
}

package cmd

import (
	"testing"
	"github.com/spf13/viper"
	"github.com/drausin/libri/libri/common/id"
	lauthor "github.com/drausin/libri/libri/author"
	"math/rand"
	"io"
	"github.com/drausin/libri/libri/common/logging"
	"github.com/stretchr/testify/assert"
	"bytes"
	"github.com/pkg/errors"
)

func TestIOTester_test_ok(t *testing.T) {
	viper.Set(nEntriesFlag, 1)
	aud := &fixedAuthorUploaderDownloader{
		rng: rand.New(rand.NewSource(0)),
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
			rng: rand.New(rand.NewSource(0)),
			uploaded: make(map[string]io.WriterTo),
		},
		ad: &fixedAuthorUploaderDownloader{downloadErr: errors.New("some download err")},
	}
	err = iot2.test(nil, logger)
	assert.NotNil(t, err)

	iot3 := &ioTesterImpl{
		au: &fixedAuthorUploaderDownloader{
			rng: rand.New(rand.NewSource(0)),
			uploaded: make(map[string]io.WriterTo),
		},
		ad: &fixedAuthorUploaderDownloader{
			rng: rand.New(rand.NewSource(0)),
			uploaded: make(map[string]io.WriterTo),
		},
	}
	err = iot3.test(nil, logger)
	assert.NotNil(t, err)
}

type fixedAuthorUploaderDownloader struct {
	rng *rand.Rand
	uploaded map[string]io.WriterTo
	uploadErr error
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


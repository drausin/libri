package page

import (
	"github.com/drausin/libri/libri/author/io/comp"
	"bytes"
	"fmt"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/librarian/api"
	"math/rand"
	"testing"
	"github.com/drausin/libri/libri/author/io/common"
)

const (
	KB = 1024
	MB = 1024 * KB
)

var (
	smallUncompressedSizes = []int{32, 64, 128, 256}
	mediumUncompressedSizes = []int{2048, 4096, 8192}
	largeUncompressedSizes = []int{256 * KB, 512 * KB, MB}
	extraLargeUncompressedSizes = []int{4 * MB, 8 * MB, 16 * MB}
)

var benchmarkCases = []struct {
	name              string
	codec             comp.Codec
	uncompressedSizes []int
}{
	{"small+none", comp.NoneCodec, smallUncompressedSizes},
	{"small+gzip", comp.GZIPCodec, smallUncompressedSizes},
	{"medium+none", comp.NoneCodec, mediumUncompressedSizes},
	{"medium+gzip", comp.GZIPCodec, mediumUncompressedSizes},
	{"large+none", comp.NoneCodec, largeUncompressedSizes},
	{"large+gzip", comp.GZIPCodec, largeUncompressedSizes},
	{"xlarge+none", comp.NoneCodec, extraLargeUncompressedSizes},
	{"xlarge+gzip", comp.GZIPCodec, extraLargeUncompressedSizes},
}

func BenchmarkPaginate(b *testing.B) {
	for _, c := range benchmarkCases {
		b.Run(c.name, func(b *testing.B) { benchmarkPaginate(b, c.uncompressedSizes, c.codec) })
	}
}

func BenchmarkUnpaginate(b *testing.B) {
	for _, c := range benchmarkCases {
		b.Run(c.name, func(b *testing.B) { benchmarkUnpaginate(b, c.uncompressedSizes, c.codec) })
	}
}

func benchmarkPaginate(b *testing.B, uncompressedSizes []int, codec comp.Codec) {
	rng := rand.New(rand.NewSource(0))
	keys := enc.NewPseudoRandomEEK(rng)
	authorPub := api.RandBytes(rng, api.ECPubKeyLength)
	encrypter, err := enc.NewEncrypter(keys)
	maybePanic(err)
	pageSize := DefaultSize

	uncompressedBytes := make([][]byte, len(uncompressedSizes))
	totBytes := int64(0)
	for i, uncompressedSize := range uncompressedSizes {
		uncompressedBytes[i] = common.NewCompressableBytes(rng, uncompressedSize).Bytes()
		totBytes += int64(uncompressedSize)
	}

	b.SetBytes(totBytes)
	for n := 0; n < b.N; n++ {
		for i := range uncompressedSizes {
			pagesChan := make(chan *api.Page, 10)
			paginator, err := NewPaginator(pagesChan, encrypter, keys, authorPub, pageSize)
			maybePanic(err)

			compressor, err := comp.NewCompressor(bytes.NewBuffer(uncompressedBytes[i]), codec,
				keys, comp.DefaultBufferSize)
			maybePanic(err)

			_, err = paginator.ReadFrom(compressor)
			maybePanic(err)
			close(pagesChan)

		}
	}
}

func benchmarkUnpaginate(b *testing.B, uncompressedSizes []int, codec comp.Codec) {
	rng := rand.New(rand.NewSource(0))
	keys := enc.NewPseudoRandomEEK(rng)
	authorPub := api.RandBytes(rng, api.ECPubKeyLength)
	encrypter, err := enc.NewEncrypter(keys)
	maybePanic(err)
	decrypter, err := enc.NewDecrypter(keys)
	maybePanic(err)
	pageSize := DefaultSize

	uncompressedBytes := make([][]byte, len(uncompressedSizes))
	pages := make([][]*api.Page, len(uncompressedSizes))
	totBytes := int64(0)
	for i, uncompressedSize := range uncompressedSizes {
		pagesChan := make(chan *api.Page, 10)  // max uncompressed size < assumes 10 * pageSize
		pages[i] = make([]*api.Page, 0)
		paginator, err := NewPaginator(pagesChan, encrypter, keys, authorPub, pageSize)
		maybePanic(err)

		uncompressedBytes[i] = common.NewCompressableBytes(rng, uncompressedSize).Bytes()
		compressor, err := comp.NewCompressor(bytes.NewBuffer(uncompressedBytes[i]), codec, keys,
			comp.DefaultBufferSize)
		maybePanic(err)

		_, err = paginator.ReadFrom(compressor)
		maybePanic(err)
		close(pagesChan)

		for page := range pagesChan {
			pages[i] = append(pages[i], page)
		}
		totBytes += int64(uncompressedSize)
	}

	b.SetBytes(totBytes)
	for n := 0; n < b.N; n++ {
		for i, uncompressedSize := range uncompressedSizes {
			decompressed := new(bytes.Buffer)
			decompressor, err := comp.NewDecompressor(decompressed, codec, keys,
				comp.DefaultBufferSize)
			maybePanic(err)

			inPages := make(chan *api.Page, 10)  // max uncompressed size < assumes 10 * pageSize
			unpaginator, err := NewUnpaginator(inPages, decrypter, keys)
			maybePanic(err)

			go func() {
				// fill inPages chan w/ pages
				for _, page := range pages[i] {
					inPages <- page
				}
				close(inPages)
			}()

			_, err = unpaginator.WriteTo(decompressor)
			maybePanic(err)

			// basic sanity check
			if decompressed.Len() != uncompressedSizes[i] {
				panic(fmt.Errorf("decompressed size (%d) does not equal decompressed size (%d)",
					decompressed.Len(), uncompressedSize))
			}
		}
	}
}

func maybePanic(err error) {
	if err != nil {
		panic(err)
	}
}
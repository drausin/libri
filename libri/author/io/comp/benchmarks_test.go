package comp

import (
	"bytes"
	"io"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/author/io/common"
	"github.com/drausin/libri/libri/author/io/enc"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/drausin/libri/libri/librarian/api"
)

const (
	KB = 1024
	MB = 1024 * KB
)

var (
	smallUncompressedSizes      = []int{128}
	mediumUncompressedSizes     = []int{KB}
	largeUncompressedSizes      = []int{MB}
	extraLargeUncompressedSizes = []int{16 * MB}
)

var benchmarkCases = []struct {
	name              string
	codec             api.CompressionCodec
	uncompressedSizes []int
}{
	{"small+none", api.CompressionCodec_NONE, smallUncompressedSizes},
	{"small+gzip", api.CompressionCodec_GZIP, smallUncompressedSizes},
	{"medium+none", api.CompressionCodec_NONE, mediumUncompressedSizes},
	{"medium+gzip", api.CompressionCodec_GZIP, mediumUncompressedSizes},
	{"large+none", api.CompressionCodec_NONE, largeUncompressedSizes},
	{"large+gzip", api.CompressionCodec_GZIP, largeUncompressedSizes},
	{"xlarge+none", api.CompressionCodec_NONE, extraLargeUncompressedSizes},
	{"xlarge+gzip", api.CompressionCodec_GZIP, extraLargeUncompressedSizes},
}

func BenchmarkCompress(b *testing.B) {
	for _, c := range benchmarkCases {
		b.Run(c.name, func(b *testing.B) { benchmarkCompress(b, c.uncompressedSizes, c.codec) })
	}
}

func BenchmarkDecompress(b *testing.B) {
	for _, c := range benchmarkCases {
		b.Run(c.name, func(b *testing.B) { benchmarkDecompress(b, c.uncompressedSizes, c.codec) })
	}
}

func benchmarkCompress(b *testing.B, uncompressedSizes []int, codec api.CompressionCodec) {
	b.StopTimer()
	rng := rand.New(rand.NewSource(0))
	keys := enc.NewPseudoRandomEEK(rng)
	uncompressedBufferSize := int(DefaultBufferSize)

	// get compressed bytes
	uncompressedBytes := make([][]byte, len(uncompressedSizes))
	totBytes := int64(0)
	for i, uncompressedSize := range uncompressedSizes {
		uncompressedBytes[i] = common.NewCompressableBytes(rng, uncompressedSize).Bytes()
		totBytes += int64(len(uncompressedBytes[i]))
	}

	b.SetBytes(totBytes)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		for _, uncompressed := range uncompressedBytes {
			compressor, err := NewCompressor(bytes.NewBuffer(uncompressed), codec, keys,
				uint32(uncompressedBufferSize))
			errors.MaybePanic(err)
			compressed := new(bytes.Buffer)
			n1 := uncompressedBufferSize
			buf := make([]byte, uncompressedBufferSize)
			for n1 == uncompressedBufferSize {
				n1, err = compressor.Read(buf)
				if err != io.EOF {
					errors.MaybePanic(err)
				}
				compressed.Write(buf[:n1])
			}
		}
	}
}

func benchmarkDecompress(b *testing.B, uncompressedSizes []int, codec api.CompressionCodec) {
	b.StopTimer()
	rng := rand.New(rand.NewSource(0))
	keys := enc.NewPseudoRandomEEK(rng)
	uncompressedBufferSize := int(DefaultBufferSize)

	// get compressed bytes
	compressedBytes := make([][]byte, len(uncompressedSizes))
	totBytes := int64(0)
	for i, uncompressedSize := range uncompressedSizes {
		uncompressed := common.NewCompressableBytes(rng, uncompressedSize)
		compressor, err := NewCompressor(uncompressed, codec, keys, uint32(uncompressedBufferSize))
		errors.MaybePanic(err)
		compressed := new(bytes.Buffer)
		n1 := uncompressedBufferSize
		buf := make([]byte, uncompressedBufferSize)
		for n1 == uncompressedBufferSize {
			n1, err = compressor.Read(buf)
			if err != io.EOF {
				errors.MaybePanic(err)
			}
			compressed.Write(buf[:n1])
		}
		compressedBytes[i] = compressed.Bytes()
		totBytes += int64(uncompressedSize)
	}

	b.SetBytes(totBytes)
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		for _, compressed := range compressedBytes {
			uncompressed := new(bytes.Buffer)
			decompressor, err := NewDecompressor(uncompressed, codec, keys,
				uint32(uncompressedBufferSize))
			errors.MaybePanic(err)

			_, err = decompressor.Write(compressed)
			errors.MaybePanic(err)

			err = decompressor.Close()
			errors.MaybePanic(err)
		}
	}
}

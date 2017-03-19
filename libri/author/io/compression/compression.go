package compression

import (
	"compress/gzip"
	"fmt"
	"io"
	"mime"
)

// Codec is a compression codec.
type Codec string

const (
	// NoneCodec indicates no compression.
	NoneCodec    Codec = "none"

	// GZIPCodec indicates gzip compression.
	GZIPCodec    Codec = "gzip"

	// DefaultCodec defines the default compression scheme.
	DefaultCodec       = GZIPCodec
)

// MediaToCompressionCodec maps MIME media types to what compression codec should be used with
// them.
var MediaToCompressionCodec = map[string]Codec{
	// don't compress again since it's already compressed
	"application/x-gzip":           NoneCodec,
	"application/x-compressed":     NoneCodec,
	"application/x-zip-compressed": NoneCodec,
	"application/zip":              NoneCodec,
}

// NewCompressor creates a new io.WriteCloser that writes compressed bytes to the underlying
// output io.Writer. It uses rawMediaType to determine which compression codec to use.
func NewCompressor(output io.Writer, rawMediaType string) (io.WriteCloser, error) {
	codec, err := GetCompressionCodec(rawMediaType)
	if err != nil {
		return nil, err
	}

	switch codec {
	case GZIPCodec:
		// optimize for best compression to reduce network transfer volume and time (at
		// expense of more client CPU)
		return gzip.NewWriterLevel(output, gzip.BestCompression)
	case NoneCodec:
		// just pass through underlying reader
		return noOpWriteCloser{output}, nil
	default:
		panic(fmt.Errorf("unexpected codec: %s", codec))
	}
}

// NewDecompressor creates a new io.ReadCloser that reads compressed bytes from the underlying
// input io.Reader. It uses rawMediaType to determine which (de)compression codec to use.
func NewDecompressor(input io.Reader, rawMediaType string) (io.ReadCloser, error) {
	codec, err := GetCompressionCodec(rawMediaType)
	if err != nil {
		return nil, err
	}

	switch codec {
	case GZIPCodec:
		return gzip.NewReader(input)
	case NoneCodec:
		return noOpReadCloser{input}, nil
	default:
		panic(fmt.Errorf("unexpected codec: %s", codec))
	}
}

// GetCompressionCodec returns the compression codec to use given a MIME media type.
func GetCompressionCodec(mediaType string) (Codec, error) {
	if mediaType == "" {
		return DefaultCodec, nil
	}

	parsedMediaType, _, err := mime.ParseMediaType(mediaType)
	if err != nil {
		return DefaultCodec, err
	}
	if codec, in := MediaToCompressionCodec[parsedMediaType]; in {
		return codec, nil
	}

	return DefaultCodec, nil
}

// noOpReadCloser implements a no-op Close method on top of an io.Reader.
type noOpReadCloser struct {
	io.Reader
}

func (noOpReadCloser) Close() error { return nil }

// noOpWriteCloser implements a no-op Close method on top of an io.Writer.
type noOpWriteCloser struct {
	io.Writer
}

func (noOpWriteCloser) Close() error { return nil }

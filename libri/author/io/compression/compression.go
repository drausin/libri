package compression

import (
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"bytes"
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

const (
	DefaultUncompressedBufferSize = 1024 * 1024
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

type flushWriter interface {
	io.Writer
	Flush() error
}

// compressor implements io.Reader, writing compressed bytes to an internal buffer that is then
// read from
type compressor struct {
	uncompressed io.Reader
	inner flushWriter
	buf *bytes.Buffer
	uncompressedBufferSize int
}

func (c *compressor) Read(p []byte) (int, error) {
	// write compressed contents into buffer until we have enough for p
	var err error
	nMore := c.uncompressedBufferSize
	for nMore == c.uncompressedBufferSize && c.buf.Len() < len(p) {
		more := make([]byte, c.uncompressedBufferSize)
		nMore, err = c.uncompressed.Read(more)
		if err != nil && err != io.EOF {
			return 0, err
		}
		c.inner.Write(more[:nMore])
		c.inner.Flush()
	}

	// read compressed contents from buffer (written to by c.inner) into p
	n, err := c.buf.Read(p)
	if err != nil {
		return n, err
	}

	// copy remainder of existing buffer to temp buffer, truncate existing buffer, and copy
	// remainder back; this prevents existing buffer from getting too long over many sequential
	// Read() calls, at the expense of copying data back and forth
	temp := new(bytes.Buffer)
	_, err = temp.ReadFrom(c.buf)
	if err != nil {
		return n, err
	}
	c.buf.Reset()
	_, err = c.buf.ReadFrom(temp)

	return n, err
}

// NewCompressor creates a new io.Reader for the compressed contents. It uses rawMediaType to
// determine which compression codec to use.
func NewCompressor(
	uncompressed io.Reader, rawMediaType string, uncompressedBuffSize int,
) (io.Reader, error) {
	codec, err := GetCompressionCodec(rawMediaType)
	if err != nil {
		return nil, err
	}

	switch codec {
	case GZIPCodec:
		// optimize for best compression to reduce network transfer volume and time (at
		// expense of more client CPU)
		buf := new(bytes.Buffer)
		inner, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
		if err != nil {
			return nil, err
		}
		return &compressor{
			uncompressed: uncompressed,
			inner: inner,
			buf: buf,
			uncompressedBufferSize: uncompressedBuffSize,
		}, nil
	case NoneCodec:
		// just pass through uncompressed reader
		return uncompressed, nil
	default:
		panic(fmt.Errorf("unexpected codec: %s", codec))
	}
}


// NewDecompressor creates a new io.Reader for the uncompressed contents. It uses rawMediaType to
// determine which (de)compression codec to use.
func NewDecompressor(compressed io.Reader, rawMediaType string) (io.Reader, error) {
	codec, err := GetCompressionCodec(rawMediaType)
	if err != nil {
		return nil, err
	}

	switch codec {
	case GZIPCodec:
		return gzip.NewReader(compressed)
	case NoneCodec:
		return compressed, nil
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

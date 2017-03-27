package compression

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"github.com/pkg/errors"
)

// Codec is a compression codec.
type Codec string

const (
	// NoneCodec indicates no compression.
	NoneCodec Codec = "none"

	// GZIPCodec indicates gzip compression.
	GZIPCodec Codec = "gzip"

	// DefaultCodec defines the default compression scheme.
	DefaultCodec = GZIPCodec
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

// CloseWriter is an io.Writer that requires Close() to be called at the end of writing.
type CloseWriter interface {
	io.Writer

	// Close writes any outstanding contents and additional footers.
	Close() error
}

// FlushCloseWriter is a CloseWriter with a Flush() method that can be called in between writes.
type FlushCloseWriter interface {
	CloseWriter

	// Flush flushes the contents to the underlying writer.
	Flush() error
}

// compressor implements io.Reader, writing compressed bytes to an internal buffer that is then
// read from during Read calls.
type compressor struct {
	uncompressed           io.Reader
	inner                  FlushCloseWriter
	buf                    *bytes.Buffer
	uncompressedBufferSize int
}

// NewCompressor creates a new io.Reader for the compressed contents using the given compression
// codec. Larger values of uncompressedBufferSize will result in fewer calls to the uncompressed
// io.Reader, at the expense of reading more than is needed into the internal buffer.
func NewCompressor(uncompressed io.Reader, codec Codec, uncompressedBufferSize int) (io.Reader,
	error) {
	var err error
	var inner FlushCloseWriter
	buf := new(bytes.Buffer)

	switch codec {
	case GZIPCodec:
		// optimize for best compression to reduce network transfer volume and time (at
		// expense of more client CPU)
		inner, err = gzip.NewWriterLevel(buf, gzip.BestCompression)
	case NoneCodec:
		// just pass through uncompressed reader
		return uncompressed, nil
	default:
		panic(fmt.Errorf("unexpected codec: %s", codec))
	}

	if err != nil {
		return nil, err
	}
	return &compressor{
		uncompressed: uncompressed,
		inner:        inner,
		buf:          buf,
		uncompressedBufferSize: uncompressedBufferSize,
	}, nil
}

// Read reads compressed contents into p from the underling uncompressed io.Reader.
func (c *compressor) Read(p []byte) (int, error) {
	// write compressed contents into buffer until we have enough for p
	for c.buf.Len() < len(p) {
		more := make([]byte, c.uncompressedBufferSize)
		nMore, err := c.uncompressed.Read(more)
		if err != nil && err != io.EOF {
			return 0, err
		}
		if _, err = c.inner.Write(more[:nMore]); err != nil {
			return 0, err
		}
		if err = c.inner.Flush(); err != nil {
			return 0, err
		}

		if nMore < c.uncompressedBufferSize {
			// break if no more uncompressed data to read
			if err = c.inner.Close(); err != nil {
				return 0, err
			}
			break
		}
	}

	// read compressed contents from buffer (written to by c.inner) into p
	n, err := c.buf.Read(p)
	if err != nil {
		return n, err
	}

	err = trimBuffer(c.buf)
	return n, err
}

// trimBuffer trims the read part of a bytes.Buffer by copying the remainder of existing buffer to
// a temp buffer, truncating the existing buffer, and coping the remainder back; this prevents
// the existing buffer from getting too long over many sequential Read() calls, at the expense of
// copying data back and forth.
func trimBuffer(buf *bytes.Buffer) error {
	temp := new(bytes.Buffer)
	_, err := temp.ReadFrom(buf)
	if err != nil {
		return err
	}
	buf.Reset()
	_, err = buf.ReadFrom(temp)
	return err
}

// decompressor implements CloseWriter, writing compressed contents to an internal buffer from which
// uncompressed contents are read and written to the underlying uncompressed io.Writer.
type decompressor struct {
	uncompressed           io.Writer
	inner                  io.Reader
	codec                  Codec
	buf                    *bytes.Buffer
	closed bool
	uncompressedBufferSize int
}

// NewDecompressor creates a new CloseWriter decompressor.
func NewDecompressor(uncompressed io.Writer, codec Codec, uncompressedBufferSize int) (
	CloseWriter, error) {
	return &decompressor{
		uncompressed: uncompressed,
		inner:        nil,
		codec:        codec,
		buf:          new(bytes.Buffer),
		closed: false,
		uncompressedBufferSize: uncompressedBufferSize,
	}, nil
}

// newInnerDecompressor creates a new io.Reader given the codec.
func newInnerDecompressor(buf io.Reader, codec Codec) (io.Reader, error) {
	switch codec {
	case GZIPCodec:
		return gzip.NewReader(buf)
	case NoneCodec:
		return buf, nil
	default:
		panic(fmt.Errorf("unexpected codec: %s", codec))
	}
}

// Write decompressed p and writes its contents to the underlying uncompressed io.Writer.
func (d *decompressor) Write(p []byte) (int, error) {
	if d.closed {
		return 0, errors.New("decompressor is closed")
	}

	// write compressed contents to buffer
	n, err := d.buf.Write(p)
	if err != nil {
		return n, err
	}

	// init inner reader if needed
	if d.inner == nil {
		d.inner, err = newInnerDecompressor(d.buf, d.codec)
		if err != nil {
			return n, err
		}
	}

	// use inner reader to read compressed chunks from the buffer and then write them to
	// uncompressed io.Writer
	for d.buf.Len() > d.uncompressedBufferSize {
		_, err = d.writeUncompressed()
		if err != nil {
			return n, err
		}
	}

	err = trimBuffer(d.buf)
	return n, err
}

// Close writes any remaining contents to the underlying uncompressed io.Writer.
func (d *decompressor) Close() error {
	var err error
	for d.buf.Len() > 0 {
		_, err = d.writeUncompressed()
		if err != nil {
			return err
		}
	}
	d.closed = true
	return nil
}

func (d *decompressor) writeUncompressed() (int, error) {
	more := make([]byte, d.uncompressedBufferSize)
	nMore, err := d.inner.Read(more)
	if err != nil && err != io.EOF {
		return nMore, err
	}
	_, err = d.uncompressed.Write(more[:nMore])
	return nMore, err
}


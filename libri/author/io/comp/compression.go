package comp

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime"

	"github.com/drausin/libri/libri/author/io/enc"
	"errors"
)

// Codec is a comp.codec.
type Codec string

const (
	// NoneCodec indicates no comp.
	NoneCodec Codec = "none"

	// GZIPCodec indicates gzip comp.
	GZIPCodec Codec = "gzip"

	// DefaultCodec defines the default comp.scheme.
	DefaultCodec = GZIPCodec

	// MinBufferSize is the minimum size of the uncompressed buffer used by the
	// compressor and decompressor.
	MinBufferSize = uint32(64)

	// DefaultBufferSize is the default size of the uncompressed buffer used by
	// the compressor and decompressor.
	DefaultBufferSize = uint32(1024)
)

// ErrBufferSizeTooSmall indicates when the max page size is too small (often because it is zero).
var ErrBufferSizeTooSmall = fmt.Errorf("buffer size is below %d byte minimum", MinBufferSize)

// MediaToCompressionCodec maps MIME media types to what comp.codec should be used with
// them.
var MediaToCompressionCodec = map[string]Codec{
	// don't compress again since it's already compressed
	"application/x-gzip":           NoneCodec,
	"application/x-compressed":     NoneCodec,
	"application/x-zip-compressed": NoneCodec,
	"application/zip":              NoneCodec,
}

// GetCompressionCodec returns the comp.codec to use given a MIME media type.
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

// noOpFlushCloseWriter wraps an io.Writer and implements no-op Flush() and Close() methods.
type noOpFlushCloseWriter struct {
	inner io.Writer
}

func (n *noOpFlushCloseWriter) Write(p []byte) (int, error) {
	return n.inner.Write(p)
}

func (n *noOpFlushCloseWriter) Flush() error {
	return nil
}

func (n *noOpFlushCloseWriter) Close() error {
	return nil
}

// Compressor is an io.Reader with an enc.MAC on the uncompressed bytes.
type Compressor interface {
	io.Reader

	// UncompressedMAC is the MAC for the uncompressed bytes.
	UncompressedMAC() enc.MAC
}

// compressor implements io.Reader, writing compressed bytes to an internal buffer that is then
// read from during Read calls.
type compressor struct {
	uncompressed           io.Reader
	inner                  FlushCloseWriter
	buf                    *bytes.Buffer
	uncompressedMAC        enc.MAC
	uncompressedBufferSize uint32
}

// NewCompressor creates a new Compressor for the compressed contents using the given compression
// codec. Larger values of uncompressedBufferSize will result in fewer calls to the uncompressed
// io.Reader, at the expense of reading more than is needed into the internal buffer.
func NewCompressor(
	uncompressed io.Reader, codec Codec, keys *enc.Keys, uncompressedBufferSize uint32,
) (Compressor, error) {
	var err error
	var inner FlushCloseWriter
	buf := new(bytes.Buffer)

	switch codec {
	case GZIPCodec:
		// optimize for best comp.to reduce network transfer volume and time (at
		// expense of more client CPU)
		inner, err = gzip.NewWriterLevel(buf, gzip.BestCompression)
	case NoneCodec:
		// inner, err = gzip.NewWriterLevel(buf, gzip.NoCompression)
		inner = &noOpFlushCloseWriter{buf}
	default:
		panic(fmt.Errorf("unexpected codec: %s", codec))
	}

	if err != nil {
		return nil, err
	}
	if uncompressedBufferSize < MinBufferSize {
		return nil, ErrBufferSizeTooSmall
	}
	return &compressor{
		uncompressed:           uncompressed,
		inner:                  inner,
		buf:                    buf,
		uncompressedMAC:        enc.NewHMAC(keys.HMACKey),
		uncompressedBufferSize: uncompressedBufferSize,
	}, nil
}

// Read reads compressed contents into p from the underling uncompressed io.Reader.
func (c *compressor) Read(p []byte) (int, error) {
	// write compressed contents into buffer until we have enough for p
	for c.buf.Len() < len(p) {
		more := make([]byte, int(c.uncompressedBufferSize))
		nMore, err := c.uncompressed.Read(more)
		if err != nil && err != io.EOF {
			return 0, err
		}
		if _, err = c.inner.Write(more[:nMore]); err != nil {
			return 0, err
		}
		if _, err := c.uncompressedMAC.Write(more[:nMore]); err != nil {
			return 0, err
		}
		if err = c.inner.Flush(); err != nil {
			return 0, err
		}

		if nMore < int(c.uncompressedBufferSize) {
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

func (c *compressor) UncompressedMAC() enc.MAC {
	return c.uncompressedMAC
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

// Decompressor is a CloseWriter with an enc.MAC on the uncompressed bytes.
type Decompressor interface {
	CloseWriter

	// UncompressedMAC is the MAC for the uncompressed bytes.
	UncompressedMAC() enc.MAC
}

// decompressor implements CloseWriter, writing compressed contents to an internal buffer from which
// uncompressed contents are read and written to the underlying uncompressed io.Writer.
type decompressor struct {
	uncompressed           io.Writer
	uncompressedMAC        enc.MAC
	inner                  io.Reader
	codec                  Codec
	buf                    *bytes.Buffer
	closed                 bool
	uncompressedBufferSize uint32
}

// NewDecompressor creates a new Decompressor instance.
func NewDecompressor(
	uncompressed io.Writer, codec Codec, keys *enc.Keys, uncompressedBufferSize uint32,
) (Decompressor, error) {
	if uncompressedBufferSize < MinBufferSize {
		return nil, ErrBufferSizeTooSmall
	}
	return &decompressor{
		uncompressed:           uncompressed,
		inner:                  nil,
		codec:                  codec,
		buf:                    new(bytes.Buffer),
		closed:                 false,
		uncompressedMAC:        enc.NewHMAC(keys.HMACKey),
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
		return 0, errorsNew("decompressor is closed")
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
	for d.buf.Len() > int(d.uncompressedBufferSize) {
		_, err = d.writeUncompressed()
		if err != nil {
			return n, err
		}
	}

	err = trimBuffer(d.buf)
	return n, err
}

func (d *decompressor) writeUncompressed() (int, error) {
	more := make([]byte, d.uncompressedBufferSize)
	nMore, err := d.inner.Read(more)
	if err != nil && err != io.EOF {
		return nMore, err
	}
	if _, err := d.uncompressedMAC.Write(more[:nMore]); err != nil {
		return nMore, err
	}
	_, err = d.uncompressed.Write(more[:nMore])
	return nMore, err
}

func (d *decompressor) UncompressedMAC() enc.MAC {
	return d.uncompressedMAC
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

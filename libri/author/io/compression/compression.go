package compression

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
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

type innerCompressor interface {
	io.Writer
	Flush() error
	Close() error
	FooterSize() int
}

type gzipInnerCompressor struct {
	*gzip.Writer
}

// FooterSize is the number of footer bytes during a close operation. See CRC32 and ISIZE
// sections of http://www.zlib.org/rfc-gzip.html.
func (gzipInnerCompressor) FooterSize() int { return 8 }



// compressor implements io.Reader, writing compressed bytes to an internal buffer that is then
// read from
type compressor struct {
	uncompressed           io.Reader
	inner                  innerCompressor
	buf                    *bytes.Buffer
	uncompressedBufferSize int
}

// NewCompressor creates a new io.Reader for the compressed contents. It uses rawMediaType to
// determine which compression codec to use.
func NewCompressor(
	uncompressed io.Reader, rawMediaType string, uncompressedBufferSize int,
) (io.Reader, error) {
	codec, err := GetCompressionCodec(rawMediaType)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	var inner innerCompressor
	switch codec {
	case GZIPCodec:
		// optimize for best compression to reduce network transfer volume and time (at
		// expense of more client CPU)
		var c *gzip.Writer
		c, err = gzip.NewWriterLevel(buf, gzip.BestCompression)
		inner = gzipInnerCompressor{c}
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

func (c *compressor) Read(p []byte) (int, error) {
	// write compressed contents into buffer until we have enough for p
	for c.buf.Len() < len(p) {
		more := make([]byte, c.uncompressedBufferSize)
		nMore, err := c.uncompressed.Read(more)
		if err != nil && err != io.EOF {
			return 0, err
		}
		c.inner.Write(more[:nMore])
		c.inner.Flush()
		if nMore < c.uncompressedBufferSize {
			// break if no more uncompressed data to read
			c.inner.Close()
			break
		}
	}

	// read compressed contents from buffer (written to by c.inner) into p
	n, err := c.buf.Read(p)
	if err != nil {
		return n, err
	}

	// copy remainder of existing buffer to temp buffer, truncate existing buffer, and copy
	// remainder back; this prevents existing buffer from getting too long over many sequential
	// Read() calls, at the expense of copying data back and forth
	err = trimBuffer(c.buf)

	return n, err
}

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

type FlushWriter interface {
	io.Writer
	Flush() error
}

type decompressor struct {
	uncompressed           io.Writer
	inner                  io.Reader
	codec                  Codec
	buf                    *bytes.Buffer
	uncompressedBufferSize int
}

func NewDecompressor(
	uncompressed io.Writer, rawMediaType string, uncompressedBufferSize int,
) (FlushWriter, error) {
	codec, err := GetCompressionCodec(rawMediaType)
	if err != nil {
		return nil, err
	}
	return &decompressor{
		uncompressed: uncompressed,
		inner:        nil,
		codec:        codec,
		buf:          new(bytes.Buffer),
		uncompressedBufferSize: uncompressedBufferSize,
	}, nil
}

func newInnerDecompressor(buf *bytes.Buffer, codec Codec) (io.Reader, error) {
	switch codec {
	case GZIPCodec:
		return gzip.NewReader(buf)
	case NoneCodec:
		return buf, nil
	default:
		panic(fmt.Errorf("unexpected codec: %s", codec))
	}
}

func (d *decompressor) Write(p []byte) (int, error) {

	// write compressed contents to buffer
	n, err := d.buf.Write(p)
	if err != nil {
		return n, err
	}
	if d.inner == nil {
		d.inner, err = newInnerDecompressor(d.buf, d.codec)
		if err != nil {
			return n, err
		}
	}

	// use inner reader to read compressed chunks from the buffer and then write them to
	// uncompressed io.Writer
	for d.buf.Len() > d.uncompressedBufferSize {
		_, err := d.writeUncompressed()
		if err != nil {
			return n, err
		}
	}

	err = trimBuffer(d.buf)
	return n, nil

}

func (d *decompressor) Flush() error {
	var err error
	for d.buf.Len() > 0 {
		_, err = d.writeUncompressed()
		if err != nil {
			return err
		}
	}
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

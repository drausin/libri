package compression

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"log"
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
	for c.buf.Len() < len(p) - c.inner.FooterSize() {
		more := make([]byte, c.uncompressedBufferSize)
		nMore, err := c.uncompressed.Read(more)
		log.Printf("nMore: %v, err: %v", nMore, err)
		if err != nil && err != io.EOF {
			return 0, err
		}
		c.inner.Write(more[:nMore])
		c.inner.Flush()
		if nMore < c.uncompressedBufferSize {
			// break if no more uncompressed data to read
			log.Printf("compressor internal buf len pre-close: %d", c.buf.Len())
			c.inner.Close()
			log.Printf("compressor internal buf len post-close: %d", c.buf.Len())
			break
		}
		log.Printf("compressor internal buf len: %d", c.buf.Len())
	}

	// read compressed contents from buffer (written to by c.inner) into p
	n, err := c.buf.Read(p)
	log.Printf("internal compressed buf read: n: %v, err: %v", n, err)
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

func writeNBytes(
	from io.Reader, w io.Writer, wBuf *bytes.Buffer, n int, tol float32,
) (int, error) {
	nLB := int(float32(n) * tol)
	nLeft := nLB - wBuf.Len()
	writeRatio := float32(1.0)
	for nLeft > 0 {
		log.Printf("nLeft: %d", nLeft)
		nextN := int(float32(nLeft) * writeRatio)
		log.Printf("nextN: %d", nextN)
		more := make([]byte, nextN)
		nRead, err := from.Read(more)
		log.Printf("nRead: %d", nRead)
		if err != nil {
			return wBuf.Len(), err
		}

		bufPreWriteLen := wBuf.Len()
		_, err = w.Write(more)
		if err != nil {
			return wBuf.Len(), err
		}
		log.Printf("postWriteLen: %d", wBuf.Len())
		if wBuf.Len() > bufPreWriteLen {
			writeRatio = float32(nRead) / float32(wBuf.Len() - bufPreWriteLen)
			log.Printf("writeRatio: %d", writeRatio)
		}
		nLeft = nLB - wBuf.Len()
	}

	if wBuf.Len() > n {
		return wBuf.Len(), fmt.Errorf("overshot writing [%d, %d] bytes when actually " +
			"wrote %d", nLB, n, wBuf.Len())
	}

	return wBuf.Len(), nil
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
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
) (io.Writer, error) {
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
	log.Printf("decompressor wrote %d compressed bytes to internal buffer", n)
	if err != nil {
		return n, err
	}
	if d.inner == nil {
		d.inner, err = newInnerDecompressor(d.buf, d.codec)
		log.Print("inner decompressor created")
		if err != nil {
			return n, err
		}
	}

	// use inner reader to read compressed chunks from the buffer and then write them to
	// uncompressed io.Writer
	nMore := d.uncompressedBufferSize
	for nMore == d.uncompressedBufferSize {
		more := make([]byte, d.uncompressedBufferSize)
		log.Printf("buf len 1: %d", d.buf.Len())
		nMore, err = d.inner.Read(more)
		log.Printf("buf len 2: %d", d.buf.Len())
		log.Printf("nMore: %d, err: %v", nMore, err)
		if err != nil && err != io.EOF {
			log.Print("returning error")
			return n, err
		}
		_, err := d.uncompressed.Write(more[:nMore])
		log.Printf("uncompressed len: %d", d.uncompressed.(*bytes.Buffer).Len())
		if err != nil {
			return n, err
		}
	}

	//err = trimBuffer(d.buf)
	return n, nil

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

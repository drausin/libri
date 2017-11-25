package common

import (
	"bytes"
	"math/rand"

	"github.com/drausin/libri/libri/common/errors"
)

// NewCompressableBytes generates a buffer of repeated strings with a given length.
func NewCompressableBytes(rng *rand.Rand, size int) *bytes.Buffer {
	dict := []string{
		"these", "are", "some", "test", "words", "that", "will", "be", "compressed",
	}
	words := new(bytes.Buffer)
	for {
		word := dict[int(rng.Int31n(int32(len(dict))))] + " "
		if words.Len()+len(word) > size {
			// pad words to exact length
			_, err := words.Write(make([]byte, size-words.Len()))
			errors.MaybePanic(err)
			break
		}
		_, err := words.WriteString(word)
		errors.MaybePanic(err)
	}

	return words
}

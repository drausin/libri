package storage

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmptyChecker_Check_ok(t *testing.T) {
	kc := NewMaxLengthChecker(8)
	cases := [][]byte{
		[]byte("key"),
		[]byte("value"),
		[]byte("ns"),
		[]byte{0},
		bytes.Repeat([]byte{0}, 7),
		bytes.Repeat([]byte{255}, 8),
	}
	for _, c := range cases {
		assert.Nil(t, kc.Check(c))
	}
}

func TestEmptyChecker_Check_err(t *testing.T) {
	kc := NewMaxLengthChecker(8)
	cases := [][]byte{
		nil,
		[]byte(nil),
		[]byte{},
		bytes.Repeat([]byte{255}, 16),
	}
	for _, c := range cases {
		assert.NotNil(t, kc.Check(c))
	}
}

func TestMaxLengthChecker_Check_ok(t *testing.T) {
	kc := NewMaxLengthChecker(8)
	cases := [][]byte{
		[]byte("key"),
		[]byte("value"),
		[]byte("ns"),
		[]byte{0},
		bytes.Repeat([]byte{0}, 7),
		bytes.Repeat([]byte{255}, 8),
	}
	for _, c := range cases {
		assert.Nil(t, kc.Check(c))
	}
}

func TestMaxLengthChecker_Check_err(t *testing.T) {
	kc := NewMaxLengthChecker(8)
	cases := [][]byte{
		nil,
		[]byte(nil),
		[]byte{},
		[]byte("too long value"),
		[]byte("too long key"),
		[]byte("too long namespace"),
		bytes.Repeat([]byte{255}, 16),
	}
	for _, c := range cases {
		assert.NotNil(t, kc.Check(c))
	}
}

func TestExactLengthChecker_Check_ok(t *testing.T) {
	kc := NewExactLengthChecker(8)
	cases := [][]byte{
		[]byte("somekey1"),
		[]byte("someval1"),
		bytes.Repeat([]byte{0}, 8),
		bytes.Repeat([]byte{255}, 8),
	}
	for _, c := range cases {
		assert.Nil(t, kc.Check(c))
	}
}

func TestExactLengthChecker_Check_err(t *testing.T) {
	kc := NewExactLengthChecker(8)
	cases := [][]byte{
		nil,
		[]byte(nil),
		[]byte{},
		[]byte("short"),
		[]byte("too long value"),
		bytes.Repeat([]byte{255}, 7),
		bytes.Repeat([]byte{255}, 9),
	}
	for _, c := range cases {
		assert.NotNil(t, kc.Check(c))
	}
}

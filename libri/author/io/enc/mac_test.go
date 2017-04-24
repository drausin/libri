package enc

import (
	"testing"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
	"math/rand"
)

func TestSizeHMAC_Write(t *testing.T) {
	hmac1 := NewHMAC([]byte{1, 2, 3})
	hmac2 := NewHMAC([]byte{4, 5, 6})

	stuff := []byte{7, 8, 9}
	n1, err := hmac1.Write(stuff)
	assert.Nil(t, err)
	assert.Equal(t, len(stuff), n1)

	n2, err := hmac2.Write(stuff)
	assert.Nil(t, err)
	assert.Equal(t, len(stuff), n2)

	mac1 := hmac1.Sum(nil)
	mac2 := hmac2.Sum(nil)
	assert.NotEqual(t, mac1, mac2)
	assert.Equal(t, len(mac1), len(mac2))
}

func TestSizeHMAC_Sum(t *testing.T) {
	hmac1 := NewHMAC([]byte{1, 2, 3})
	hmac2 := NewHMAC([]byte{1, 2, 3})
	hmac3 := NewHMAC([]byte{4, 5, 6})

	stuff := []byte{7, 8, 9}
	_, err := hmac1.Write(stuff)
	assert.Nil(t, err)
	_, err = hmac2.Write(stuff)
	assert.Nil(t, err)
	_, err = hmac3.Write(stuff)
	assert.Nil(t, err)

	mac1 := hmac1.Sum(nil)
	mac2 := []byte{0, 0, 0}
	mac2 = hmac2.Sum(mac2)
	mac3 := hmac3.Sum(nil)

	assert.Equal(t, mac1, mac2[3:])
	assert.Equal(t, len(mac1), len(mac3))
	assert.Nil(t, api.ValidateHMAC256(mac1))
	assert.Nil(t, api.ValidateHMAC256(mac3))
	assert.NotEqual(t, mac1, mac3)
}

func TestSizeHMAC_Reset(t *testing.T) {
	hmac1 := NewHMAC([]byte{1, 2, 3})
	hmac2 := NewHMAC([]byte{1, 2, 3})

	stuff1 := []byte{4, 5, 6}
	_, err := hmac1.Write(stuff1)
	assert.Nil(t, err)

	// check that hmac1 and hmac2 currently have different MACs
	assert.NotEqual(t, hmac2.Sum(nil), hmac1.Sum(nil))

	// check that Reset() results in same MAC as fresh
	stuff2 := []byte{7, 8, 9}
	hmac1.Reset()
	_, err = hmac1.Write(stuff2)
	assert.Nil(t, err)
	_, err = hmac2.Write(stuff2)
	assert.Nil(t, err)
	assert.Equal(t, hmac2.Sum(nil), hmac1.Sum(nil))
}

func TestSizeHMAC_MessageSize(t *testing.T) {
	hmac1 := NewHMAC([]byte{1, 2, 3})

	stuff := []byte{7, 8, 9}
	_, err := hmac1.Write(stuff)
	assert.Nil(t, err)
	assert.Equal(t, uint64(len(stuff)), hmac1.MessageSize())

	moreStuff := []byte{10, 11, 12, 14}
	_, err = hmac1.Write(moreStuff)
	assert.Nil(t, err)
	assert.Equal(t, uint64(len(stuff)+len(moreStuff)), hmac1.MessageSize())
}

func TestHMAC(t *testing.T) {
	mac := HMAC([]byte{1, 2, 3}, []byte{4, 5, 6})
	assert.Nil(t, api.ValidateHMAC256(mac))
}

func TestCheckMACs_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := api.RandBytes(rng, 32)
	uncompressedMAC, ciphertextMAC := NewHMAC(key), NewHMAC(key)
	_, err := uncompressedMAC.Write([]byte("some uncompressed stuff"))
	assert.Nil(t, err)
	_, err = ciphertextMAC.Write([]byte("some ciphertext"))
	assert.Nil(t, err)
	md, err := api.NewEntryMetadata(
		"application/x-pdf",
		ciphertextMAC.MessageSize(),
		ciphertextMAC.Sum(nil),
		uncompressedMAC.MessageSize(),
		uncompressedMAC.Sum(nil),
	)
	assert.Nil(t, err)

	err = CheckMACs(ciphertextMAC, uncompressedMAC, md)
	assert.Nil(t, err)
}

func TestCheckMACs_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	key := api.RandBytes(rng, 32)
	uncompressedMAC, ciphertextMAC := NewHMAC(key), NewHMAC(key)
	_, err := uncompressedMAC.Write([]byte("some uncompressed stuff"))
	assert.Nil(t, err)
	_, err = ciphertextMAC.Write([]byte("some ciphertext"))
	assert.Nil(t, err)
	_, err = ciphertextMAC.Write([]byte("some ciphertext"))
	assert.Nil(t, err)
	mediaType := "application/x-pdf"

	// check ValidateMetadata error bubbles up
	md1, err := api.NewEntryMetadata(
		mediaType,
		ciphertextMAC.MessageSize(),
		ciphertextMAC.Sum(nil),
		uncompressedMAC.MessageSize(),
		uncompressedMAC.Sum(nil),
	)
	assert.Nil(t, err)
	md1.SetBytes(api.MetadataEntryCiphertextMAC, nil)  // now md1 is invalid
	err = CheckMACs(ciphertextMAC, uncompressedMAC, md1)
	assert.NotNil(t, err)

	// check errors on different ciphertext sizes
	md2, err := api.NewEntryMetadata(
		mediaType,
		1,
		ciphertextMAC.Sum(nil),
		uncompressedMAC.MessageSize(),
		uncompressedMAC.Sum(nil),
	)
	assert.Nil(t, err)
	err = CheckMACs(ciphertextMAC, uncompressedMAC, md2)
	assert.Equal(t, ErrUnexpectedCiphertextSize, err)

	// check errors on different ciphertext MACs
	md3, err := api.NewEntryMetadata(
		mediaType,
		ciphertextMAC.MessageSize(),
		api.RandBytes(rng, 32),
		uncompressedMAC.MessageSize(),
		uncompressedMAC.Sum(nil),
	)
	assert.Nil(t, err)
	err = CheckMACs(ciphertextMAC, uncompressedMAC, md3)
	assert.Equal(t, ErrUnexpectedCiphertextMAC, err)

	// check errors on different uncompressed sizes
	md4, err := api.NewEntryMetadata(
		mediaType,
		ciphertextMAC.MessageSize(),
		ciphertextMAC.Sum(nil),
		1,
		uncompressedMAC.Sum(nil),
	)
	assert.Nil(t, err)
	err = CheckMACs(ciphertextMAC, uncompressedMAC, md4)
	assert.Equal(t, ErrUnexpectedUncompressedSize, err)

	// check errors on different uncompressed MACs
	md5, err := api.NewEntryMetadata(
		mediaType,
		ciphertextMAC.MessageSize(),
		ciphertextMAC.Sum(nil),
		uncompressedMAC.MessageSize(),
		api.RandBytes(rng, 32),
	)
	assert.Nil(t, err)
	err = CheckMACs(ciphertextMAC, uncompressedMAC, md5)
	assert.Equal(t, ErrUnexpectedUncompressedMAC, err)
}

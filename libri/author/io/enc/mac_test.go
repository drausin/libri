package enc

import (
	"testing"

	"github.com/drausin/libri/libri/librarian/api"
	"github.com/stretchr/testify/assert"
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

package keychain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	veryLightScryptN = 2
	veryLightScryptP = 1
)

func TestToFromStored_ok(t *testing.T) {
	nKeys := 3
	kc1 := New(nKeys)

	auth2 := "test passphrase"
	stored1, err := EncryptToStored(kc1, auth2, veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)
	assert.Equal(t, nKeys, len(stored1.PrivateKeys))

	kc2, err := DecryptFromStored(stored1, auth2)
	assert.Nil(t, err)
	assert.Equal(t, kc1.privs, kc2.privs)
	assert.Equal(t, kc1.pubs, kc2.pubs)

	auth3 := "a different test passphrase"
	stored2, err := EncryptToStored(kc1, auth3, veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)
	assert.Equal(t, nKeys, len(stored2.PrivateKeys))
	assert.NotEqual(t, stored1, stored2)

	kc3, err := DecryptFromStored(stored2, auth3)
	assert.Nil(t, err)
	assert.Equal(t, kc1.privs, kc3.privs)
	assert.Equal(t, kc1.pubs, kc3.pubs)
}

func TestToFromStored_err(t *testing.T) {
	nKeys := 3
	kc1 := New(nKeys)

	stored1, err := EncryptToStored(kc1, "test passphrase", veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)
	assert.Equal(t, nKeys, len(stored1.PrivateKeys))

	kc2, err := DecryptFromStored(stored1, "wrong passphrase")
	assert.NotNil(t, err)
	assert.Nil(t, kc2)
}

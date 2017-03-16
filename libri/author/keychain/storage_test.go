package keychain

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

const (
	veryLightScryptN = 2
	veryLightScryptP = 1
)

func TestToFromStored(t *testing.T) {
	nKeys := 3
	kc1 := New(nKeys)

	auth2 := "test passphrase"
	stored1, err := ToStored(kc1, auth2, veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)
	assert.Equal(t, nKeys, len(stored1.KeyEncKeys))

	kc2, err := FromStored(stored1, auth2)
	assert.Nil(t, err)
	assert.Equal(t, kc1, kc2)
}

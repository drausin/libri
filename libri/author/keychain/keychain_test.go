package keychain

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSaveLoad(t *testing.T) {
	file, err := ioutil.TempFile("", "kechain-test")
	defer func() { assert.Nil(t, os.Remove(file.Name())) }()
	assert.Nil(t, file.Close())
	assert.Nil(t, err)

	kc1, auth := New(3), "test passphrase"
	err = Save(file.Name(), auth, kc1, veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)

	kc2, err := Load(file.Name(), auth)
	assert.Nil(t, err)
	assert.Equal(t, kc1, kc2)

	kc3, err := Load(file.Name(), "wrong passphrase")
	assert.NotNil(t, err)
	assert.Nil(t, kc3)

}

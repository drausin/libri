package keychain

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSave_err(t *testing.T) {
	file, err := ioutil.TempFile("", "kechain-test")
	defer func() { assert.Nil(t, os.Remove(file.Name())) }()
	assert.Nil(t, err)
	assert.Nil(t, file.Close())

	// check error from bad scrypt params bubbles up
	err = Save(file.Name(), "test", New(3), -1, -1)
	assert.NotNil(t, err)
}

func TestLoad_err(t *testing.T) {
	file, err := ioutil.TempFile("", "kechain-test")
	defer func() { assert.Nil(t, os.Remove(file.Name())) }()
	assert.Nil(t, err)
	n, err := file.Write([]byte("not a keychain"))
	assert.Nil(t, err)
	assert.NotZero(t, n)
	assert.Nil(t, file.Close())

	// check that error from unmarshalling bad file bubbles up
	kc, err := Load(file.Name(), "test")
	assert.NotNil(t, err)
	assert.Nil(t, kc)
}

func TestSaveLoad(t *testing.T) {
	file, err := ioutil.TempFile("", "kechain-test")
	defer func() { assert.Nil(t, os.Remove(file.Name())) }()
	assert.Nil(t, err)
	assert.Nil(t, file.Close())

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

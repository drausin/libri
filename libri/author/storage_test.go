package author

import (
	"testing"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"errors"
	"io/ioutil"
	"os"
	"path"
)

const (
	veryLightScryptN = 2
	veryLightScryptP = 1
)

func TestLoadOrCreateClientID_ok(t *testing.T) {

	// create new client ID
	id1, err := loadOrCreateClientID(clogging.NewDevInfoLogger(), &fixedStorerLoader{})
	assert.NotNil(t, id1)
	assert.Nil(t, err)

	// load existing
	rng := rand.New(rand.NewSource(0))
	peerID2 := ecid.NewPseudoRandom(rng)
	bytes, err := proto.Marshal(ecid.ToStored(peerID2))
	assert.Nil(t, err)

	id2, err := loadOrCreateClientID(clogging.NewDevInfoLogger(), &fixedStorerLoader{loadBytes: bytes})

	assert.Equal(t, peerID2, id2)
	assert.Nil(t, err)
}

func TestLoadOrCreatePeerID_err(t *testing.T) {
	id1, err := loadOrCreateClientID(clogging.NewDevInfoLogger(), &fixedStorerLoader{
		loadErr: errors.New("some load error"),
	})
	assert.Nil(t, id1)
	assert.NotNil(t, err)

	id2, err := loadOrCreateClientID(clogging.NewDevInfoLogger(), &fixedStorerLoader{
		loadBytes: []byte("the wrong bytes"),
	})
	assert.Nil(t, id2)
	assert.NotNil(t, err)
}

func TestSaveClientID(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	assert.Nil(t, saveClientID(&fixedStorerLoader{}, ecid.NewPseudoRandom(rng)))
}

func TestLoadKeychains(t *testing.T) {
	testKeychainDir, err := ioutil.TempDir("", "author-test-keychains")
	defer os.RemoveAll(testKeychainDir)
	assert.Nil(t, err)
	auth := "some secret passphrase"

	err = createKeychains(clogging.NewDevInfoLogger(), testKeychainDir, auth,
		veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)

	// check our keychains load properly and have the expected length
	authorKeys, selfReaderKeys, err := loadKeychains(testKeychainDir, auth)
	assert.Nil(t, err)
	assert.Equal(t, nInitialKeys, authorKeys.Len())
	assert.Equal(t, nInitialKeys, selfReaderKeys.Len())

	// delete self reader keychain to trigger error
	os.Remove(path.Join(testKeychainDir, selfReaderKeychainFilename))

	// check missing self reader keychain file triggers error
	authorKeys, selfReaderKeys, err = loadKeychains(testKeychainDir, auth)
	assert.NotNil(t, err)
	assert.Nil(t, authorKeys)
	assert.Nil(t, selfReaderKeys)

	// delete author keychain to trigger error
	os.Remove(path.Join(testKeychainDir, authorKeychainFilename))

	// check missing author keychain file triggers error
	authorKeys, selfReaderKeys, err = loadKeychains(testKeychainDir, auth)
	assert.NotNil(t, err)
	assert.Nil(t, authorKeys)
	assert.Nil(t, selfReaderKeys)
}

func TestCreateKeychains_ok(t *testing.T) {
	testKeychainDir, err := ioutil.TempDir("", "author-test-keychains")
	defer os.RemoveAll(testKeychainDir)
	assert.Nil(t, err)
	auth := "some secret passphrase"

	// check creating in existing dir is fine
	err = createKeychains(clogging.NewDevInfoLogger(), testKeychainDir, auth,
		veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)

	// check creating in new dir is fine
	testKeychainSubDir := path.Join(testKeychainDir, "sub")
	err = createKeychains(clogging.NewDevInfoLogger(), testKeychainSubDir, auth,
		veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)
}

func TestCreateKeychains_err(t *testing.T) {
	testKeychainDir, err := ioutil.TempDir("", "author-test-keychains")
	defer os.RemoveAll(testKeychainDir)
	assert.Nil(t, err)
	auth := "some secret passphrase"

	selfReaderKeysFP := path.Join(testKeychainDir, selfReaderKeychainFilename)
	err = ioutil.WriteFile(selfReaderKeysFP, []byte("some random stuff"), os.ModePerm)
	assert.Nil(t, err)

	// check create self reader keychain error bubbles up
	err = createKeychains(clogging.NewDevInfoLogger(), testKeychainDir, auth,
		veryLightScryptN, veryLightScryptP)
	assert.NotNil(t, err)

	// check create author keychain error bubbles up
	err = createKeychains(clogging.NewDevInfoLogger(), testKeychainDir, auth,
		veryLightScryptN, veryLightScryptP)
	assert.NotNil(t, err)
}

func TestCreateKeychain(t *testing.T) {
	testKeychainDir, err := ioutil.TempDir("", "author-test-keychains")
	defer os.RemoveAll(testKeychainDir)
	assert.Nil(t, err)
	authorKeychainFP := path.Join(testKeychainDir, authorKeychainFilename)
	auth := "some secret passphrase"

	err = createKeychain(clogging.NewDevInfoLogger(), authorKeychainFP, auth,
		veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)

	// check keychain file exists
	info, err := os.Stat(authorKeychainFP)
	assert.Nil(t, err)
	assert.NotNil(t, info)

	// check attempt to create keychain in same file returns error
	err = createKeychain(clogging.NewDevInfoLogger(), authorKeychainFP, auth,
		veryLightScryptN, veryLightScryptP)
	assert.Equal(t, ErrKeychainExists, err)

	// check save error bubbles up
	otherAuthorKeychainFP := path.Join(testKeychainDir, "other-author.keys")
	err = createKeychain(clogging.NewDevInfoLogger(), otherAuthorKeychainFP, auth, -1, -1)
	assert.NotNil(t, err)
}

type fixedStorerLoader struct {
	loadBytes []byte
	loadErr   error
	storeErr error
}

func (l *fixedStorerLoader) Load(key []byte) ([]byte, error) {
	return l.loadBytes, l.loadErr
}

func (l *fixedStorerLoader) Store(key []byte, value []byte) error {
	return l.storeErr
}


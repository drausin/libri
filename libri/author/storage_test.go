package author

import (
	"errors"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
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
	defer rmDir(testKeychainDir)
	assert.Nil(t, err)
	auth := "some secret passphrase"

	err = CreateKeychains(clogging.NewDevInfoLogger(), testKeychainDir, auth,
		veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)

	// check our keychains load properly and have the expected length
	authorKeys, selfReaderKeys, err := loadKeychains(testKeychainDir, auth)
	assert.Nil(t, err)
	assert.Equal(t, nInitialKeys, authorKeys.Len())
	assert.Equal(t, nInitialKeys, selfReaderKeys.Len())

	// delete self reader keychain to trigger error
	err = os.Remove(path.Join(testKeychainDir, selfReaderKeychainFilename))
	assert.Nil(t, err)

	// check missing self reader keychain file triggers error
	authorKeys, selfReaderKeys, err = loadKeychains(testKeychainDir, auth)
	assert.NotNil(t, err)
	assert.Nil(t, authorKeys)
	assert.Nil(t, selfReaderKeys)

	// delete author keychain to trigger error
	err = os.Remove(path.Join(testKeychainDir, authorKeychainFilename))
	assert.Nil(t, err)

	// check missing author keychain file triggers error
	authorKeys, selfReaderKeys, err = loadKeychains(testKeychainDir, auth)
	assert.NotNil(t, err)
	assert.Nil(t, authorKeys)
	assert.Nil(t, selfReaderKeys)
}

func TestCreateKeychains_ok(t *testing.T) {
	testKeychainDir, err := ioutil.TempDir("", "author-test-keychains")
	defer rmDir(testKeychainDir)
	assert.Nil(t, err)
	auth := "some secret passphrase"

	// check creating in existing dir is fine
	err = CreateKeychains(clogging.NewDevInfoLogger(), testKeychainDir, auth,
		veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)

	// check creating in new dir is fine
	testKeychainSubDir := path.Join(testKeychainDir, "sub")
	err = CreateKeychains(clogging.NewDevInfoLogger(), testKeychainSubDir, auth,
		veryLightScryptN, veryLightScryptP)
	assert.Nil(t, err)
}

func TestCreateKeychains_err(t *testing.T) {
	testKeychainDir, err := ioutil.TempDir("", "author-test-keychains")
	defer rmDir(testKeychainDir)
	assert.Nil(t, err)
	auth := "some secret passphrase"

	selfReaderKeysFP := path.Join(testKeychainDir, selfReaderKeychainFilename)
	err = ioutil.WriteFile(selfReaderKeysFP, []byte("some random stuff"), os.ModePerm)
	assert.Nil(t, err)

	// check create self reader keychain error bubbles up
	err = CreateKeychains(clogging.NewDevInfoLogger(), testKeychainDir, auth,
		veryLightScryptN, veryLightScryptP)
	assert.NotNil(t, err)

	// check create author keychain error bubbles up
	err = CreateKeychains(clogging.NewDevInfoLogger(), testKeychainDir, auth,
		veryLightScryptN, veryLightScryptP)
	assert.NotNil(t, err)
}

func TestCreateKeychain(t *testing.T) {
	testKeychainDir, err := ioutil.TempDir("", "author-test-keychains")
	defer rmDir(testKeychainDir)
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

func TestMissingKeychains(t *testing.T) {
	testKeychainDir, err := ioutil.TempDir("", "author-test-keychains")
	defer rmDir(testKeychainDir)
	authorKCPath := path.Join(testKeychainDir, authorKeychainFilename)
	selfReaderKCPath := path.Join(testKeychainDir, selfReaderKeychainFilename)
	assert.Nil(t, err)

	// check missing when neither file exists
	missing, err := MissingKeychains(testKeychainDir)
	assert.Nil(t, err)
	assert.True(t, missing)

	// check not missing with both files exist
	_, err = os.Create(authorKCPath)
	assert.Nil(t, err)
	_, err = os.Create(selfReaderKCPath)
	assert.Nil(t, err)
	missing, err = MissingKeychains(testKeychainDir)
	assert.Nil(t, err)
	assert.False(t, missing)

	// check error when only one file is missing
	err = os.Remove(selfReaderKCPath)
	assert.Nil(t, err)
	missing, err = MissingKeychains(testKeychainDir)
	assert.Equal(t, errMissingSelfReaderKeychain, err)
	assert.True(t, missing)

	// check error when only other file is missing
	err = os.Remove(authorKCPath)
	assert.Nil(t, err)
	_, err = os.Create(selfReaderKCPath)
	assert.Nil(t, err)
	missing, err = MissingKeychains(testKeychainDir)
	assert.Equal(t, errMissingAuthorKeychain, err)
	assert.True(t, missing)
}

func rmDir(dir string) {
	err := os.RemoveAll(dir)
	if err != nil {
		panic(err)
	}
}

type fixedStorerLoader struct {
	loadBytes []byte
	loadErr   error
	storeErr  error
}

func (l *fixedStorerLoader) Load(key []byte) ([]byte, error) {
	return l.loadBytes, l.loadErr
}

func (l *fixedStorerLoader) Store(key []byte, value []byte) error {
	return l.storeErr
}

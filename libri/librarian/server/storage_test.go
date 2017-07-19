package server

import (
	"math/rand"
	"testing"

	"errors"

	"github.com/drausin/libri/libri/common/ecid"
	clogging "github.com/drausin/libri/libri/common/logging"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func TestLoadOrCreatePeerID_ok(t *testing.T) {

	// create new peer ID
	id1, err := loadOrCreatePeerID(clogging.NewDevInfoLogger(), &fixedStorerLoader{})
	assert.NotNil(t, id1)
	assert.Nil(t, err)

	// load existing
	rng := rand.New(rand.NewSource(0))
	peerID2 := ecid.NewPseudoRandom(rng)
	bytes, err := proto.Marshal(ecid.ToStored(peerID2))
	assert.Nil(t, err)

	id2, err := loadOrCreatePeerID(clogging.NewDevInfoLogger(), &fixedStorerLoader{loadBytes: bytes})

	assert.Equal(t, peerID2, id2)
	assert.Nil(t, err)
}

func TestLoadOrCreatePeerID_err(t *testing.T) {
	id1, err := loadOrCreatePeerID(clogging.NewDevInfoLogger(), &fixedStorerLoader{
		loadErr: errors.New("some load error"),
	})
	assert.Nil(t, id1)
	assert.NotNil(t, err)

	id2, err := loadOrCreatePeerID(clogging.NewDevInfoLogger(), &fixedStorerLoader{
		loadBytes: []byte("the wrong bytes"),
	})
	assert.Nil(t, id2)
	assert.NotNil(t, err)
}

func TestSavePeerID(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	assert.Nil(t, savePeerID(&fixedStorerLoader{}, ecid.NewPseudoRandom(rng)))
}

func TestLoadOrCreateRoutingTable_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	// load stored RT
	selfID1 := ecid.NewPseudoRandom(rng)
	srt1 := &storage.RoutingTable{
		SelfId: selfID1.ID().Bytes(),
	}
	bytes, err := proto.Marshal(srt1)
	assert.Nil(t, err)

	fullLoader := &fixedStorerLoader{
		loadBytes: bytes,
	}
	rt1, err := loadOrCreateRoutingTable(clogging.NewDevInfoLogger(), fullLoader, selfID1,
		routing.NewDefaultParameters())
	assert.Equal(t, selfID1.ID(), rt1.SelfID())
	assert.Nil(t, err)

	// create new RT
	selfID2 := ecid.NewPseudoRandom(rng)
	rt2, err := loadOrCreateRoutingTable(clogging.NewDevInfoLogger(), &fixedStorerLoader{}, selfID2,
		routing.NewDefaultParameters())
	assert.Equal(t, selfID2.ID(), rt2.SelfID())
	assert.Nil(t, err)
}

func TestLoadOrCreateRoutingTable_loadErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := ecid.NewPseudoRandom(rng)

	errLoader := &fixedStorerLoader{
		loadErr: errors.New("some error during load"),
	}

	rt1, err := loadOrCreateRoutingTable(clogging.NewDevInfoLogger(), errLoader, selfID,
		routing.NewDefaultParameters())
	assert.Nil(t, rt1)
	assert.NotNil(t, err)
}

func TestLoadOrCreateRoutingTable_selfIDErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	selfID1 := ecid.NewPseudoRandom(rng)
	srt1 := &storage.RoutingTable{
		SelfId: selfID1.Bytes(),
	}
	bytes, err := proto.Marshal(srt1)
	assert.Nil(t, err)

	fullLoader := &fixedStorerLoader{
		loadBytes: bytes,
	}

	// error with conflicting/different selfID
	selfID2 := ecid.NewPseudoRandom(rng)
	rt1, err := loadOrCreateRoutingTable(clogging.NewDevInfoLogger(), fullLoader, selfID2,
		routing.NewDefaultParameters())
	assert.Nil(t, rt1)
	assert.NotNil(t, err)
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

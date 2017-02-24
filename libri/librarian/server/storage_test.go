package server

import (
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

type fixedLoader struct {
	bytes []byte
	err   error
}

func (l *fixedLoader) Load(key []byte) ([]byte, error) {
	return l.bytes, l.err
}

func TestLoadOrCreatePeerID_ok(t *testing.T) {

	// create new peer ID
	id1, err := loadOrCreatePeerID(&fixedLoader{})
	assert.NotNil(t, id1)
	assert.Nil(t, err)

	// load existing
	rng := rand.New(rand.NewSource(0))
	peerID2 := ecid.NewPseudoRandom(rng)
	bytes, err := proto.Marshal(ecid.ToStored(peerID2))
	assert.Nil(t, err)

	id2, err := loadOrCreatePeerID(&fixedLoader{
		bytes: bytes,
	})
	assert.Equal(t, peerID2, id2)
	assert.Nil(t, err)
}

func TestLoadOrCreatePeerID_err(t *testing.T) {
	id1, err := loadOrCreatePeerID(&fixedLoader{
		err: errors.New("some load error"),
	})
	assert.Nil(t, id1)
	assert.NotNil(t, err)

	id2, err := loadOrCreatePeerID(&fixedLoader{
		bytes: []byte("the wrong bytes"),
	})
	assert.Nil(t, id2)
	assert.NotNil(t, err)
}

type noOpStorer struct{}

func (s noOpStorer) Store(key []byte, value []byte) error {
	return nil
}

func TestSavePeerID(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	assert.Nil(t, savePeerID(&noOpStorer{}, ecid.NewPseudoRandom(rng)))
}

func TestLoadOrCreateRoutingTable_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	// load stored RT
	selfID1 := id.NewPseudoRandom(rng)
	srt1 := &storage.RoutingTable{
		SelfId: selfID1.Bytes(),
	}
	bytes, err := proto.Marshal(srt1)
	assert.Nil(t, err)

	fullLoader := &fixedLoader{
		bytes: bytes,
	}
	rt1, err := loadOrCreateRoutingTable(fullLoader, selfID1)
	assert.Equal(t, selfID1, rt1.SelfID())
	assert.Nil(t, err)

	// create new RT
	selfID2 := id.NewPseudoRandom(rng)
	rt2, err := loadOrCreateRoutingTable(&fixedLoader{}, selfID2)
	assert.Equal(t, selfID2, rt2.SelfID())
	assert.Nil(t, err)
}

func TestLoadOrCreateRoutingTable_loadErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	selfID := id.NewPseudoRandom(rng)

	errLoader := &fixedLoader{
		err: errors.New("some error during load"),
	}

	rt1, err := loadOrCreateRoutingTable(errLoader, selfID)
	assert.Nil(t, rt1)
	assert.NotNil(t, err)
}

func TestLoadOrCreateRoutingTable_selfIDErr(t *testing.T) {
	rng := rand.New(rand.NewSource(0))

	selfID1 := id.NewPseudoRandom(rng)
	srt1 := &storage.RoutingTable{
		SelfId: selfID1.Bytes(),
	}
	bytes, err := proto.Marshal(srt1)
	assert.Nil(t, err)

	fullLoader := &fixedLoader{
		bytes: bytes,
	}

	// error with conflicting/different selfID
	selfID2 := id.NewPseudoRandom(rng)
	rt1, err := loadOrCreateRoutingTable(fullLoader, selfID2)
	assert.Nil(t, rt1)
	assert.NotNil(t, err)
}

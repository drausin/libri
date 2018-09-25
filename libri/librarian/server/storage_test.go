package server

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/drausin/libri/libri/common/ecid"
	clogging "github.com/drausin/libri/libri/common/logging"
	cstorage "github.com/drausin/libri/libri/common/storage"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func TestLoadOrCreatePeerID_ok(t *testing.T) {

	// create new peer ID
	id1, err := loadOrCreatePeerID(clogging.NewDevInfoLogger(), &cstorage.TestSLD{})
	assert.NotNil(t, id1)
	assert.Nil(t, err)

	// load existing
	rng := rand.New(rand.NewSource(0))
	peerID2 := ecid.NewPseudoRandom(rng)
	bytes, err := proto.Marshal(ecid.ToStored(peerID2))
	assert.Nil(t, err)

	id2, err := loadOrCreatePeerID(clogging.NewDevInfoLogger(), &cstorage.TestSLD{Bytes: bytes})

	assert.Equal(t, peerID2, id2)
	assert.Nil(t, err)
}

func TestLoadOrCreatePeerID_err(t *testing.T) {
	id1, err := loadOrCreatePeerID(clogging.NewDevInfoLogger(), &cstorage.TestSLD{
		LoadErr: errors.New("some load error"),
	})
	assert.Nil(t, id1)
	assert.NotNil(t, err)

	id2, err := loadOrCreatePeerID(clogging.NewDevInfoLogger(), &cstorage.TestSLD{
		Bytes: []byte("the wrong bytes"),
	})
	assert.Nil(t, id2)
	assert.NotNil(t, err)
}

func TestSavePeerID(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	assert.Nil(t, savePeerID(&cstorage.TestSLD{}, ecid.NewPseudoRandom(rng)))
}

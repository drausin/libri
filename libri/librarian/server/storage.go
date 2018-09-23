package server

import (
	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/golang/protobuf/proto"
	"go.uber.org/zap"
)

// logger keys
const (
	// LoggerPeerID is a peer ID.
	LoggerPeerID = "peerId"

	// NumPeers is a number of peers.
	NumPeers = "numPeers"

	// NumBuckets is a number of routing table buckets.
	NumBuckets = "numBuckets"
)

var (
	peerIDKey = []byte("PeerID")
)

func loadOrCreatePeerID(logger *zap.Logger, nsl storage.StorerLoader) (ecid.ID, error) {
	bytes, err := nsl.Load(peerIDKey)
	if err != nil {
		logger.Error("error loading peer ID", zap.Error(err))
		return nil, err
	}

	if bytes != nil {
		// return saved PeerID
		stored := &ecid.ECDSAPrivateKey{}
		if err := proto.Unmarshal(bytes, stored); err != nil {
			return nil, err
		}
		peerID, err := ecid.FromStored(stored)
		if err != nil {
			logger.Error("error deserializing peer ID keys", zap.Error(err))
			return nil, err
		}
		logger.Info("loaded exsting peer ID", zap.String(LoggerPeerID, peerID.String()))
		return peerID, nil
	}

	// return new PeerID
	peerID := ecid.NewRandom()
	logger.Info("created new peer ID", zap.String(LoggerPeerID, peerID.String()))
	return peerID, savePeerID(nsl, peerID)
}

func savePeerID(ns storage.Storer, peerID ecid.ID) error {
	bytes, err := proto.Marshal(ecid.ToStored(peerID))
	if err != nil {
		return err
	}
	return ns.Store(peerIDKey, bytes)
}

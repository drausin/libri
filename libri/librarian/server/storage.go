package server

import (
	"errors"
	"fmt"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/drausin/libri/libri/common/storage"
	"github.com/drausin/libri/libri/librarian/server/routing"
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

func loadOrCreatePeerID(logger *zap.Logger, nsl storage.NamespaceSL) (ecid.ID, error) {
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

func savePeerID(ns storage.NamespaceStorer, peerID ecid.ID) error {
	bytes, err := proto.Marshal(ecid.ToStored(peerID))
	if err != nil {
		return err
	}
	return ns.Store(peerIDKey, bytes)
}

func loadOrCreateRoutingTable(logger *zap.Logger, nl storage.NamespaceLoader, selfID ecid.ID,
	params *routing.Parameters) (routing.Table, error) {
	rt, err := routing.Load(nl, params)
	if err != nil {
		logger.Error("error loading routing table", zap.Error(err))
		return nil, err
	}

	if rt != nil {
		if selfID.ID().Cmp(rt.SelfID()) != 0 {
			msg := fmt.Sprintf("selfID (%v) of loaded routing table does not match "+
				"Librarian selfID (%v)", rt.SelfID(), selfID)
			err := errors.New(msg)
			logger.Error(msg, zap.Error(err))
			return nil, err
		}
		logger.Info("loaded routing table",
			zap.Int(NumPeers, rt.NumPeers()),
			zap.Int(NumBuckets, rt.NumBuckets()),
		)
		return rt, nil
	}

	defer logger.Info("created new routing table")
	return routing.NewEmpty(selfID.ID(), params), nil
}

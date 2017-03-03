package server

import (
	"fmt"

	cid "github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"github.com/drausin/libri/libri/librarian/server/routing"
	"github.com/drausin/libri/libri/librarian/server/storage"
	"github.com/gogo/protobuf/proto"
	"go.uber.org/zap"
)

var (
	// logger keys
	LoggerPeerID = "peerId"

	peerIDKey = []byte("PeerID")
)

func loadOrCreatePeerID(nl storage.NamespaceLoader, logger *zap.Logger) (ecid.ID, error) {
	bytes, err := nl.Load(peerIDKey)
	if err != nil {
		logger.Error("error loading peer ID", zap.Error(err))
		return nil, err
	}

	if bytes != nil {
		// return saved PeerID
		stored := &storage.ECID{}
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
	return peerID, nil

}

func savePeerID(ns storage.NamespaceStorer, peerID ecid.ID) error {
	bytes, err := proto.Marshal(ecid.ToStored(peerID))
	if err != nil {
		return err
	}
	return ns.Store(peerIDKey, bytes)
}

func loadOrCreateRoutingTable(nl storage.NamespaceLoader, selfID cid.ID) (routing.Table, error) {
	rt, err := routing.Load(nl)
	if err != nil {
		return nil, err
	}

	if rt != nil {
		if selfID.Cmp(rt.SelfID()) != 0 {
			return nil, fmt.Errorf("selfID (%v) of loaded routing table does not "+
				"match Librarian selfID (%v)", rt.SelfID(), selfID)
		}
		return rt, nil
	}

	return routing.NewEmpty(selfID), nil
}

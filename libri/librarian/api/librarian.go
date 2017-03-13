package api

import (
	"crypto/sha256"
	cid "github.com/drausin/libri/libri/common/id"
	"github.com/golang/protobuf/proto"
)

func GetKey(doc *Document) (cid.ID, error) {
	bytes, err := proto.Marshal(doc)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(bytes)
	return cid.FromBytes(hash[:]), nil
}

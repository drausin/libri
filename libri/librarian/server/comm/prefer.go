package comm

import (
	"github.com/drausin/libri/libri/common/id"
	"github.com/drausin/libri/libri/librarian/api"
)

// Preferer judges whether one peer is preferable over another.
type Preferer interface {

	// Prefer indicates whether peer 1 should be preferred over peer 2 when prioritization
	// is necessary.
	Prefer(peerID1, peerID2 id.ID) bool
}

// NewRpPreferer returns a Preferer that prefers peers with a larger number of successful
// Verify or Find responses.
func NewRpPreferer(getter QueryGetter) Preferer {
	return &rpPreferer{getter}
}

type rpPreferer struct {
	getter QueryGetter
}

func (p *rpPreferer) Prefer(peerID1, peerID2 id.ID) bool {
	nRps1 := p.getter.Get(peerID1, api.Verify)[Response][Success].Count
	nRps2 := p.getter.Get(peerID2, api.Verify)[Response][Success].Count
	if nRps1 == 0 || nRps2 == 0 {
		nRps1 = p.getter.Get(peerID1, api.Find)[Response][Success].Count
		nRps2 = p.getter.Get(peerID2, api.Find)[Response][Success].Count
	}
	return nRps1 > nRps2
}

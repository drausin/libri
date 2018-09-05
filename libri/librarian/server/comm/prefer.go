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

// NewVerifyRpPreferer returns a Preferer that prefers peers with a larger number of successful
// Verify responses.
func NewVerifyRpPreferer(getter QueryGetter) Preferer {
	return &verifyRpPreferer{getter}
}

type verifyRpPreferer struct {
	getter QueryGetter
}

func (p *verifyRpPreferer) Prefer(peerID1, peerID2 id.ID) bool {
	nRps1 := p.getter.Get(peerID1, api.Verify)[Response][Success].Count
	nRps2 := p.getter.Get(peerID2, api.Verify)[Response][Success].Count
	return nRps1 > nRps2
}

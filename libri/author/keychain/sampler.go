package keychain

import (
	"github.com/drausin/libri/libri/common/ecid"
)

type Sampler interface {
	Sample() (ecid.ID, error)
}

package signature

import (
	"github.com/gogo/protobuf/proto"
	"crypto/ecdsa"
	"crypto/sha256"
	"github.com/dgrijalva/jwt-go"
	"regexp"
	"encoding/base64"
	"fmt"
	"github.com/drausin/libri/libri/librarian/server/ecid"
	"bytes"
)

const (
	ContextKey = "signature"
)

// regex pattern for a base-64 url-encoded string for a 256-bit number
var b64url256bit *regexp.Regexp

func init() {
	// base-64 encoded 256-bit values have 43 chars followed by an =
	b64url256bit, _ = regexp.Compile("^[A-Za-z0-9\\-_]{43}=$")
}

type SignatureClaims struct {
	// a base-64-url encoded string of the hash of the message being signed
	Hash string `json:"hash"`
}

// Valid returns whether the claim is valid or invalid via an error.
func (c *SignatureClaims) Valid() error {
	// check that message hash looks like a base-64-url encoded string
	if !b64url256bit.MatchString(c.Hash) {
		return fmt.Errorf("%v does not looks like a base-64-url encoded 32-byte number",
			c.Hash)
	}
	return nil
}

func NewSignatureClaims(hash [sha256.Size]byte) *SignatureClaims {
	return &SignatureClaims {
		Hash: base64.URLEncoding.EncodeToString(hash[:]),
	}
}

type Signer interface {
	// Sign returns the signature on the message.
	Sign(m proto.Message) (string, error)
}

type ecdsaSigner struct {
	key *ecdsa.PrivateKey
}

func NewSigner(key *ecdsa.PrivateKey) Signer {
	return &ecdsaSigner{key}
}

func (s *ecdsaSigner) Sign(m proto.Message) (string, error) {
	hash, err := hashMessage(m)
	if err != nil {
		return "", err
	}

	// create token
	token := jwt.NewWithClaims(jwt.SigningMethodES256, NewSignatureClaims(hash))

	// sign with key, yield encoded token string like XXXXXX.YYYYYY.ZZZZZZ
	return token.SignedString(s.key)
}

type Verifier interface {
	// Verify verifies that the encoded token is well formed and has been signed by the peer.
	Verify(encToken string, peerID ecid.ID, m proto.Message) error
}

type ecsdaVerifier struct {}

func NewVerifier() Verifier {
	return &ecsdaVerifier{}
}

func (v *ecsdaVerifier) Verify(encToken string, peerID ecid.ID, m proto.Message) error {
	token, err := jwt.ParseWithClaims(encToken, &SignatureClaims{}, func(token *jwt.Token) (
		interface{}, error) {
		return &peerID.Key().PublicKey, nil
	})
	if err != nil {
		// received error when parsing claims or verifying signature
		return err
	}

	claims, ok := token.Claims.(*SignatureClaims)
	if !ok {
		return fmt.Errorf("token claims %v are not expected SignatureClaims", token.Claims)
	}

	return verifyMessageHash(m, claims.Hash)
}

func verifyMessageHash(m proto.Message, encClaimedHash string) error {
	messageHash, err := hashMessage(m)
	if err != nil {
		return err
	}
	claimedHash, err := base64.URLEncoding.DecodeString(encClaimedHash)
	if err != nil {
		return err
	}
	if !bytes.Equal(messageHash[:], claimedHash) {
		return fmt.Errorf("token claim hash %064X does not match actual hash %064X",
			claimedHash, messageHash[:])
	}
	return nil
}

func hashMessage(m proto.Message) ([sha256.Size]byte, error) {
	buf, err := proto.Marshal(m)
	if err != nil {
		var empty [sha256.Size]byte
		return empty, err
	}
	return sha256.Sum256(buf), nil
}
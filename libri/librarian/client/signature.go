package client

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"regexp"

	"github.com/dgrijalva/jwt-go"
	"github.com/drausin/libri/libri/common/errors"
	"github.com/golang/protobuf/proto"
)

// regex pattern for a base-64 url-encoded string for a 256-bit number
var b64url256bit *regexp.Regexp

func init() {
	// base-64 encoded 256-bit values have 43 chars followed by an =
	var err error
	b64url256bit, err = regexp.Compile(`^[A-Za-z0-9\-_]{43}=$`)
	errors.MaybePanic(err)
}

// Claims holds the claims associated with a message signature.
type Claims struct {
	// Hash is the base-64-url encoded string of the hash of the message being signed
	Hash string `json:"hash"`
}

// Valid returns whether the claim is valid or invalid via an error.
func (c *Claims) Valid() error {
	// check that message hash looks like a base-64-url encoded string
	if !b64url256bit.MatchString(c.Hash) {
		return fmt.Errorf("%v does not look like a base-64-url encoded 32-byte number",
			c.Hash)
	}
	return nil
}

// NewSignatureClaims creates a new SignatureClaims instance with the given message hash.
func NewSignatureClaims(hash [sha256.Size]byte) *Claims {
	return &Claims{
		Hash: base64.URLEncoding.EncodeToString(hash[:]),
	}
}

// Signer can sign a message.
type Signer interface {
	// Sign returns the signature (in the form of an encoded json web token) on the message.
	Sign(m proto.Message) (string, error)
}

type emptySigner struct{}

func (emptySigner) Sign(m proto.Message) (string, error) {
	return "", nil
}

// NewEmptySigner returns a Signer than returns an empty string signature instead of actually
// signing the message.
func NewEmptySigner() Signer {
	return emptySigner{}
}

type ecdsaSigner struct {
	key *ecdsa.PrivateKey
}

// NewECDSASigner returns a new Signer instance using the given private key.
func NewECDSASigner(key *ecdsa.PrivateKey) Signer {
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

// Verifier verifies the signature on a message.
type Verifier interface {
	// Verify verifies that the encoded token is well formed and has been signed by the peer.
	Verify(encToken string, fromPubKey *ecdsa.PublicKey, m proto.Message) error
}

type ecsdaVerifier struct{}

// NewVerifier creates a new Verifier instance.
func NewVerifier() Verifier {
	return &ecsdaVerifier{}
}

func (v *ecsdaVerifier) Verify(encToken string, fromPubKey *ecdsa.PublicKey,
	m proto.Message) error {
	token, err := jwt.ParseWithClaims(encToken, &Claims{}, func(token *jwt.Token) (
		interface{}, error) {
		return fromPubKey, nil
	})
	if err != nil {
		// received error when parsing claims or verifying signature
		return err
	}

	claims, ok := token.Claims.(*Claims)
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
		return fmt.Errorf("token claim hash %064x does not match actual hash %064x",
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

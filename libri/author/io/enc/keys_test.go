package enc

import (
	"math/rand"
	"testing"

	"crypto/ecdsa"
	"crypto/elliptic"

	"github.com/drausin/libri/libri/common/ecid"
	"github.com/stretchr/testify/assert"
	"github.com/drausin/libri/libri/librarian/api"
)

func TestNewKEK_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	authorPriv := ecid.NewPseudoRandom(rng)
	readerPriv := ecid.NewPseudoRandom(rng)

	k1, err := NewKEK(authorPriv.Key(), &readerPriv.Key().PublicKey)
	assert.Nil(t, err)

	assert.NotNil(t, k1.AESKey)
	assert.NotNil(t, k1.IV)
	assert.NotNil(t, k1.HMACKey)

	// check that first 8 bytes of adjacent fields are different
	assert.NotEqual(t, k1.AESKey[:8], k1.IV[:8])
	assert.NotEqual(t, k1.IV[:8], k1.HMACKey[:8])

	k2, err := NewKEK(readerPriv.Key(), &authorPriv.Key().PublicKey)
	assert.Nil(t, err)

	// check that ECDH shared secred + HKDF create same keys
	assert.Equal(t, k1, k2)
}

func TestNewKEK_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	privOffCurve, err := ecdsa.GenerateKey(elliptic.P256(), rng)
	assert.Nil(t, err)
	privOnCurve := ecid.NewPseudoRandom(rng)

	// check that off-curve private key results in error
	k1, err := NewKEK(privOffCurve, &privOnCurve.Key().PublicKey)
	assert.NotNil(t, err)
	assert.Nil(t, k1)

	// check that off-surve public key results in error
	k2, err := NewKEK(privOnCurve.Key(), &privOffCurve.PublicKey)
	assert.NotNil(t, err)
	assert.Nil(t, k2)
}

func TestKEK_EncryptDecrypt_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	kek, _, _ := NewPseudoRandomKEK(rng)
	eek1 := NewPseudoRandomEEK(rng)

	eekCiphertext, eekCiphertextMAC, err := kek.Encrypt(eek1)
	assert.Nil(t, err)
	eek2, err := kek.Decrypt(eekCiphertext, eekCiphertextMAC)

	assert.Equal(t, eek1, eek2)
}

func TestKEK_Encrypt_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	kek := &KEK{} // nil AESkey will cause newGCMCipher to error
	eek1 := NewPseudoRandomEEK(rng)

	eekCiphertext, eekCiphertextMAC, err := kek.Encrypt(eek1)
	assert.NotNil(t, err)
	assert.Nil(t, eekCiphertext)
	assert.Nil(t, eekCiphertextMAC)
}

func TestKEK_Decrypt_err(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	eekCiphertext := api.RandBytes(rng, api.EEKLength)
	eekCiphertextMAC := api.RandBytes(rng, api.HMAC256Length)

	kek1 := &KEK{} // nil AESkey will cause newGCMCipher to error
	eek, err := kek1.Decrypt(eekCiphertext, eekCiphertextMAC)
	assert.NotNil(t, err)
	assert.Nil(t, eek)

	kek2, _, _ := NewPseudoRandomKEK(rng)
	eek, err = kek2.Decrypt(eekCiphertext, eekCiphertextMAC)
	assert.NotNil(t, err)
	assert.Nil(t, eek)

	kek3, _, _ := NewPseudoRandomKEK(rng)
	kek3.IV = nil  // will cause gcmCiper.Open to error
	eek, err = kek3.Decrypt(eekCiphertext, eekCiphertextMAC)
	assert.NotNil(t, err)
	assert.Nil(t, eek)
}

func TestMarshallUnmarshallKEK_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	kek1, _, _ := NewPseudoRandomKEK(rng)
	kek2, err := UnmarshalKEK(MarshalKEK(kek1))
	assert.Nil(t, err)
	assert.Equal(t, kek1, kek2)
}

func TestUnmarshalKEK_err(t *testing.T) {
	_, err := UnmarshalEEK([]byte{})
	assert.NotNil(t, err)
}

func TestNewEEK_ok(t *testing.T) {
	k1, err := NewEEK()
	assert.Nil(t, err)

	assert.NotNil(t, k1.AESKey)
	assert.NotNil(t, k1.PageIVSeed)
	assert.NotNil(t, k1.HMACKey)
	assert.NotNil(t, k1.MetadataIV)

	// check that first 8 bytes of adjacent fields are different
	assert.NotEqual(t, k1.AESKey[:8], k1.PageIVSeed[:8])
	assert.NotEqual(t, k1.PageIVSeed[:8], k1.HMACKey[:8])
	assert.NotEqual(t, k1.HMACKey[:8], k1.MetadataIV[:8])
}

func TestMarshallUnmarshall_ok(t *testing.T) {
	rng := rand.New(rand.NewSource(0))
	eek1 := NewPseudoRandomEEK(rng)
	eek2, err := UnmarshalEEK(MarshalEEK(eek1))
	assert.Nil(t, err)
	assert.Equal(t, eek1, eek2)
}

func TestUnmarshal_err(t *testing.T) {
	_, err := UnmarshalEEK([]byte{})
	assert.NotNil(t, err)
}

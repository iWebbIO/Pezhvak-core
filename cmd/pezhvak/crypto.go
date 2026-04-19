package core

import (
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/nacl/box"
)

// GenerateIdentity creates a new Curve25519 keypair for the user.
func GenerateIdentity() (pubKey *[32]byte, privKey *[32]byte, err error) {
	return box.GenerateKey(rand.Reader)
}

// EncryptPayload encrypts data using NaCl box (Curve25519 + XSalsa20 + Poly1305).
func EncryptPayload(privKey *[32]byte, pubKey *[32]byte, data []byte) ([]byte, error) {
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}

	// box.Seal appends the encrypted data to the nonce.
	// The resulting slice is [nonce][ciphertext]
	return box.Seal(nonce[:], data, &nonce, pubKey, privKey), nil
}

// DecryptPayload decrypts data encrypted via EncryptPayload.
func DecryptPayload(privKey *[32]byte, pubKey *[32]byte, data []byte) ([]byte, error) {
	if len(data) < 24 {
		return nil, errors.New("ciphertext too short")
	}

	var nonce [24]byte
	copy(nonce[:], data[:24])
	ciphertext := data[24:]

	plaintext, ok := box.Open(nil, ciphertext, &nonce, pubKey, privKey)
	if !ok {
		return nil, errors.New("decryption failed (invalid key or tampered data)")
	}

	return plaintext, nil
}
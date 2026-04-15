package core

import (
	"crypto/rand"
	"errors"
	"io"

	"golang.org/x/crypto/nacl/box"
)

// GenerateIdentity creates a new public/private keypair for a peer.
func GenerateIdentity() (publicKey, privateKey *[32]byte, err error) {
	return box.GenerateKey(rand.Reader)
}

// EncryptPayload encrypts a message using Curve25519, XSalsa20, and Poly1305.
func EncryptPayload(senderPrivKey, recipientPubKey *[32]byte, plaintext []byte) ([]byte, error) {
	var nonce [24]byte
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}

	// Seal appends the encrypted payload to the nonce
	encrypted := box.Seal(nonce[:], plaintext, &nonce, recipientPubKey, senderPrivKey)
	return encrypted, nil
}

// DecryptPayload verifies and decrypts a received message.
func DecryptPayload(recipientPrivKey, senderPubKey *[32]byte, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < 24 {
		return nil, errors.New("ciphertext too short")
	}

	var nonce [24]byte
	copy(nonce[:], ciphertext[:24])

	decrypted, ok := box.Open(nil, ciphertext[24:], &nonce, senderPubKey, recipientPrivKey)
	if !ok {
		return nil, errors.New("decryption failed or message forged")
	}

	return decrypted, nil
}
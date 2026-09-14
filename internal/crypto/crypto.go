// Package crypto wraps AES-256-GCM as the sole encryption primitive used by mlp.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

const (
	KeySize   = 32 // AES-256
	NonceSize = 12 // standard GCM nonce size
)

// ErrAuthFailed means the ciphertext failed authentication: wrong key,
// corrupted data, or tampering. It never indicates a partial/garbled
// plaintext was produced — GCM refuses to return one.
var ErrAuthFailed = errors.New("authentication failed: data is corrupted, tampered with, or was encrypted with a different key")

func newGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != KeySize {
		return nil, errors.New("crypto: key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Encrypt seals plaintext under key using the given nonce. The nonce must
// be exactly NonceSize bytes and must never be reused for the same key.
func Encrypt(key, nonce, plaintext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("crypto: nonce must be 12 bytes")
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nil
}

// Decrypt opens ciphertext under key and nonce. Returns ErrAuthFailed if
// authentication fails for any reason.
func Decrypt(key, nonce, ciphertext []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("crypto: nonce must be 12 bytes")
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrAuthFailed
	}
	return plaintext, nil
}

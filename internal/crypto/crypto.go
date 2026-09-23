// SPDX-License-Identifier: GPL-3.0-or-later

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
	TagSize   = 16 // GCM authentication tag appended to the ciphertext
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
//
// aad (additional authenticated data) is bound to the resulting tag without
// being encrypted: Decrypt must be given the exact same aad bytes or
// authentication fails. Passing the .mlp header's own bytes as aad is what
// stops the header (extension, timestamps, the compressed flag) from being
// tampered with independently of the ciphertext — a plain nil aad only
// protects plaintext, not the fields describing how to interpret it. nil is
// fine when there's nothing to bind (e.g. a version 1 header, which
// predates this).
func Encrypt(key, nonce, plaintext, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("crypto: nonce must be 12 bytes")
	}
	return gcm.Seal(nil, nonce, plaintext, aad), nil
}

// Decrypt opens ciphertext under key, nonce and aad (see Encrypt). Returns
// ErrAuthFailed if authentication fails for any reason, including aad not
// matching what was passed to Encrypt.
func Decrypt(key, nonce, ciphertext, aad []byte) ([]byte, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("crypto: nonce must be 12 bytes")
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, ErrAuthFailed
	}
	return plaintext, nil
}

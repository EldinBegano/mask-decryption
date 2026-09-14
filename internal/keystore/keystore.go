// Package keystore manages the single symmetric keyfile and its
// counter-based nonce state, located automatically in the OS config
// directory (or MLP_CONFIG_DIR, if set) with no path input from the user.
package keystore

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	KeySize   = 32 // AES-256
	NonceSize = 12

	dirName     = "mask-decryption"
	keyFileName = "keyfile"
	counterName = "counter"
)

// ErrKeyfileMissing is returned by LoadKey when no keyfile exists yet.
var ErrKeyfileMissing = errors.New("keyfile not found — run 'mlp encrypt' or 'mlp keygen' first")

// ConfigDir resolves the directory holding the keyfile and counter state:
// $MLP_CONFIG_DIR if set, otherwise the OS user config dir.
func ConfigDir() (string, error) {
	if dir := os.Getenv("MLP_CONFIG_DIR"); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, dirName), nil
}

func keyPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, keyFileName), nil
}

func counterPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, counterName), nil
}

// Exists reports whether a keyfile is already present.
func Exists() (bool, error) {
	p, err := keyPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// LoadKey loads the existing key, or ErrKeyfileMissing if none exists.
func LoadKey() ([]byte, error) {
	p, err := keyPath()
	if err != nil {
		return nil, err
	}
	key, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrKeyfileMissing
		}
		return nil, err
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("keyfile at %s is corrupt (wrong size)", p)
	}
	return key, nil
}

// LoadOrCreateKey loads the key, generating one on first use. created
// reports whether a brand-new key was just generated.
func LoadOrCreateKey() (key []byte, created bool, err error) {
	key, err = LoadKey()
	if err == nil {
		return key, false, nil
	}
	if !errors.Is(err, ErrKeyfileMissing) {
		return nil, false, err
	}
	key, err = Keygen()
	if err != nil {
		return nil, false, err
	}
	return key, true, nil
}

// Keygen creates a new random key and resets the nonce counter to zero,
// overwriting any existing key. Data encrypted under the old key becomes
// permanently unreadable.
func Keygen() ([]byte, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	kp, err := keyPath()
	if err != nil {
		return nil, err
	}
	if err := writeFileFsync(kp, key, 0o600); err != nil {
		return nil, err
	}

	cp, err := counterPath()
	if err != nil {
		return nil, err
	}
	if err := writeCounter(cp, 0); err != nil {
		return nil, err
	}

	return key, nil
}

// NextNonce returns the next nonce to use for encryption. The counter is
// advanced and fsynced to disk *before* the value is handed back, so a
// crash between allocating and using a nonce can never cause reuse under
// the same key.
//
// If the counter state is missing or corrupt (but the keyfile is still
// present), NextNonce falls back to a random nonce for this one operation
// and reports that via fellBack so the caller can warn loudly.
func NextNonce() (nonce [NonceSize]byte, fellBack bool, err error) {
	cp, err := counterPath()
	if err != nil {
		return nonce, false, err
	}

	cur, rerr := readCounter(cp)
	if rerr != nil {
		if _, err := rand.Read(nonce[:]); err != nil {
			return nonce, true, err
		}
		return nonce, true, nil
	}

	next := cur + 1
	if err := writeCounter(cp, next); err != nil {
		return nonce, false, err
	}

	// First 4 bytes zero, last 8 bytes the counter: 2^64 values, never wraps
	// in practice.
	binary.BigEndian.PutUint32(nonce[0:4], 0)
	binary.BigEndian.PutUint64(nonce[4:12], next)
	return nonce, false, nil
}

// KeyfileSHA256 returns a hex SHA-256 digest of the current keyfile, so a
// user can verify a backup copy matches.
func KeyfileSHA256() (string, error) {
	p, err := keyPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Export copies the keyfile and counter state into destDir (created if
// needed), bundled together so a later Import continues the nonce counter
// correctly instead of risking reuse.
func Export(destDir string) error {
	ok, err := Exists()
	if err != nil {
		return err
	}
	if !ok {
		return ErrKeyfileMissing
	}

	kp, err := keyPath()
	if err != nil {
		return err
	}
	cp, err := counterPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return err
	}
	if err := copyFile(kp, filepath.Join(destDir, keyFileName)); err != nil {
		return err
	}
	if _, err := os.Stat(cp); err == nil {
		if err := copyFile(cp, filepath.Join(destDir, counterName)); err != nil {
			return err
		}
	} else if err := writeCounter(filepath.Join(destDir, counterName), 0); err != nil {
		return err
	}
	return nil
}

// Import installs a keyfile+counter backup from srcDir into the config
// dir, overwriting whatever is there. Callers should confirm with the
// user first when a keyfile already exists.
func Import(srcDir string) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	kp, err := keyPath()
	if err != nil {
		return err
	}
	cp, err := counterPath()
	if err != nil {
		return err
	}
	if err := copyFile(filepath.Join(srcDir, keyFileName), kp); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(srcDir, counterName), cp); err != nil {
		return err
	}
	return nil
}

func readCounter(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	if len(data) != 8 {
		return 0, fmt.Errorf("counter file at %s is corrupt (wrong size)", path)
	}
	return binary.BigEndian.Uint64(data), nil
}

func writeCounter(path string, v uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, v)
	return writeFileFsync(path, buf, 0o600)
}

func writeFileFsync(path string, data []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return writeFileFsync(dst, data, 0o600)
}

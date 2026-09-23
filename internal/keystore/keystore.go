// SPDX-License-Identifier: GPL-3.0-or-later

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

	dirName            = "mask-decryption"
	keyFileName        = "keyfile"
	counterName        = "counter"
	oldKeyFileName     = "keyfile.old"
	oldCounterFileName = "counter.old"
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

// Keygen creates a new random key, overwriting any existing one. Data
// encrypted under the old key can no longer be decrypted with the new one
// active — but the old key itself is preserved as keyfile.old (see
// OldKeyPath) first, so that outcome is recoverable by hand, not permanent.
// This mirrors the safety net Rotation.Commit gives mlp rotate.
func Keygen() ([]byte, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	kp, err := keyPath()
	if err != nil {
		return nil, err
	}
	if err := backupIfExists(kp, filepath.Join(dir, oldKeyFileName)); err != nil {
		return nil, err
	}

	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := writeFileFsync(kp, key, 0o600); err != nil {
		return nil, err
	}

	// The counter is a never-lowered high-water-mark across every key this
	// config dir has ever had (see Rotation.Commit), not reset here: if
	// keyfile.old is ever manually restored, resuming encryption under it
	// then can never reuse a nonce it already used before this Keygen call.
	// A missing/corrupt counter (including a genuinely first-ever key)
	// starts fresh at 0, same as before this safety net existed.
	cp, err := counterPath()
	if err != nil {
		return nil, err
	}
	if _, err := readCounter(cp); err != nil {
		if err := writeCounter(cp, 0); err != nil {
			return nil, err
		}
	}

	return key, nil
}

// backupIfExists copies src to dst if src exists, leaving dst untouched if
// src doesn't. Used to preserve whatever key/counter is about to be
// replaced.
func backupIfExists(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return writeFileAtomic(dst, data, 0o600)
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
	return nonceFromCounter(next), false, nil
}

// nonceFromCounter puts v in the last 8 bytes of a zeroed 12-byte nonce:
// 2^64 values, never wraps in practice.
func nonceFromCounter(v uint64) (nonce [NonceSize]byte) {
	binary.BigEndian.PutUint64(nonce[4:12], v)
	return nonce
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
// dir, overwriting whatever is there. Whatever key was previously active is
// preserved first as keyfile.old (with its own counter as counter.old, so
// the two can be restored together as a matched, safe-to-resume pair) —
// importing the wrong backup by mistake is then recoverable, not permanent.
// Callers should still confirm with the user first when a keyfile already
// exists.
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
	if err := backupIfExists(kp, filepath.Join(dir, oldKeyFileName)); err != nil {
		return err
	}
	if err := backupIfExists(cp, filepath.Join(dir, oldCounterFileName)); err != nil {
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

// Rotation is a new key being prepared in memory. Nothing touches disk
// until Commit, so a rotation that fails while re-encrypting files leaves
// the active key and counter untouched.
type Rotation struct {
	key  []byte
	used uint64
}

// BeginRotation generates a fresh random key, held in memory only.
func BeginRotation() (*Rotation, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return &Rotation{key: key}, nil
}

// Key returns the new key being prepared.
func (r *Rotation) Key() []byte { return r.key }

// NextNonce returns the next counter-based nonce under the new key.
func (r *Rotation) NextNonce() [NonceSize]byte {
	r.used++
	return nonceFromCounter(r.used)
}

// Commit makes the new key active. The old key is kept as keyfile.old so
// files stranded by a crash mid-rotation stay recoverable.
//
// The counter is written first, and never lowered: whichever of the two
// writes a crash interrupts, the surviving key still has a counter at or
// above every nonce already used under it.
//
// If the counter state is missing or corrupt, Commit can't know how high
// the old key's true usage was, so it can only make the counter safe for
// the new key (which is all it needs to be) — not for resuming the old
// key later from keyfile.old. fellBack reports this so the caller can warn
// loudly, same as NextNonce's identical fallback does.
func (r *Rotation) Commit() (fellBack bool, err error) {
	old, err := LoadKey()
	if err != nil {
		return false, err
	}
	dir, err := ConfigDir()
	if err != nil {
		return false, err
	}
	kp, err := keyPath()
	if err != nil {
		return false, err
	}
	cp, err := counterPath()
	if err != nil {
		return false, err
	}

	cur, cerr := readCounter(cp)
	if cerr != nil {
		cur = 0
		fellBack = true
	}
	if err := writeCounter(cp, max(cur, r.used)); err != nil {
		return fellBack, err
	}
	if err := writeFileAtomic(filepath.Join(dir, oldKeyFileName), old, 0o600); err != nil {
		return fellBack, err
	}
	return fellBack, writeFileAtomic(kp, r.key, 0o600)
}

// OldKeyPath is where Commit, Keygen and Import keep the previous key.
func OldKeyPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, oldKeyFileName), nil
}

// OldCounterPath is where Import keeps the previous counter, matched to
// the key at OldKeyPath so the two can be restored together safely.
func OldCounterPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, oldCounterFileName), nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := writeFileFsync(tmp, data, perm); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

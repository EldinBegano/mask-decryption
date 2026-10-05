// SPDX-License-Identifier: GPL-3.0-or-later

// Package keystore manages the single symmetric keyfile and its
// counter-based nonce state, located automatically in the OS config
// directory (or MLP_CONFIG_DIR, if set) with no path input from the user.
//
// Every read-modify-write of the keyfile or counter holds the config
// directory's lock file, so two mlp processes running at once (two
// terminals, a parallel script, the CLI next to the GUI) can never hand out
// the same nonce or swap the key out from under each other.
package keystore

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/EldinBegano/mask-decryption/internal/fsutil"
)

const (
	KeySize   = 32 // AES-256
	NonceSize = 12

	dirName            = "mask-decryption"
	keyFileName        = "keyfile"
	counterName        = "counter"
	oldKeyFileName     = "keyfile.old"
	oldCounterFileName = "counter.old"
	lockFileName       = "lock"
)

// ErrKeyfileMissing is returned by LoadKey when no keyfile exists yet.
var ErrKeyfileMissing = errors.New("keyfile not found — run 'mlp encrypt' or 'mlp keygen' first")

// ErrKeyChanged means the active keyfile was replaced (by mlp keygen, rotate
// or keyfile import, in another process) while an operation that started
// with the old key was still running. Nothing was written for it.
var ErrKeyChanged = errors.New("the keyfile was replaced while this was running (mlp keygen, rotate or keyfile import elsewhere?) — run it again")

// ErrBackupExists is returned by Export when the destination already holds
// a backup of a different key and replacing it wasn't asked for.
var ErrBackupExists = errors.New("destination already holds a backup of a different key")

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

// lockConfig creates the config dir if needed and takes its lock. Callers
// must call unlock when done.
func lockConfig() (dir string, unlock func(), err error) {
	dir, err = ConfigDir()
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", nil, err
	}
	unlock, err = fsutil.Lock(filepath.Join(dir, lockFileName))
	if err != nil {
		return "", nil, err
	}
	return dir, unlock, nil
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
	if !errors.Is(err, ErrKeyfileMissing) {
		return key, false, err
	}

	// Checked again under the lock: two first-ever encrypts racing each
	// other must not both generate a key, or the second would replace the
	// key the first is already encrypting with.
	dir, unlock, err := lockConfig()
	if err != nil {
		return nil, false, err
	}
	defer unlock()
	key, err = LoadKey()
	if !errors.Is(err, ErrKeyfileMissing) {
		return key, false, err
	}
	key, err = keygen(dir)
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
	dir, unlock, err := lockConfig()
	if err != nil {
		return nil, err
	}
	defer unlock()
	return keygen(dir)
}

// keygen is Keygen with the config dir's lock already held.
func keygen(dir string) ([]byte, error) {
	kp := filepath.Join(dir, keyFileName)
	if err := backupIfExists(kp, filepath.Join(dir, oldKeyFileName)); err != nil {
		return nil, err
	}

	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := fsutil.WriteFileAtomic(kp, key, 0o600); err != nil {
		return nil, err
	}

	// The counter is a never-lowered high-water-mark across every key this
	// config dir has ever had (see Rotation.Commit), not reset here: if
	// keyfile.old is ever manually restored, resuming encryption under it
	// then can never reuse a nonce it already used before this Keygen call.
	// A missing/corrupt counter (including a genuinely first-ever key)
	// starts fresh at 0, same as before this safety net existed.
	cp := filepath.Join(dir, counterName)
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
	return fsutil.WriteFileAtomic(dst, data, 0o600)
}

// NextNonce returns the next nonce to use for encrypting under key. The
// counter is advanced and made durable *before* the value is handed back,
// so a crash between allocating and using a nonce can never cause reuse
// under the same key.
//
// The whole read-advance-write runs under the config dir's lock, so
// concurrent encrypts each get their own value. It also checks that key is
// still the active keyfile: a caller holding a key in memory (a batch run)
// must not take values from a counter that, after a keyfile import, belongs
// to a different key (ErrKeyChanged).
//
// If the counter state is missing or corrupt (but the keyfile is still
// present), NextNonce falls back to a random nonce for this one operation
// and reports that via fellBack so the caller can warn loudly.
func NextNonce(key []byte) (nonce [NonceSize]byte, fellBack bool, err error) {
	dir, unlock, err := lockConfig()
	if err != nil {
		return nonce, false, err
	}
	defer unlock()

	active, err := LoadKey()
	if err != nil {
		return nonce, false, err
	}
	if !bytes.Equal(active, key) {
		return nonce, false, ErrKeyChanged
	}

	cp := filepath.Join(dir, counterName)
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

// unusedCounterStart picks a counter for a key whose counter state was
// lost: a random point in [2^63, 2^63+2^62). The lost counter only ever
// counted up from 0, one per file encrypted, so it was nowhere near 2^63 —
// counting on from here can't repeat a nonce it handed out, and leaves 2^62
// values of room. (Random fallback nonces from NextNonce collide with
// counter nonces only if their first 4 bytes happen to be zero, 1 in 2^32,
// and then only if the remaining 8 bytes match exactly.)
func unusedCounterStart() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return 1<<63 | binary.BigEndian.Uint64(b[:])>>2, nil
}

// Export copies the keyfile and counter state into destDir (created if
// needed), bundled together so a later Import continues the nonce counter
// correctly instead of risking reuse.
//
// If destDir already holds a backup of a *different* key, Export refuses
// with ErrBackupExists unless replace is set: overwriting it would silently
// lose what may be that key's only backup. A backup of the same key is
// simply refreshed.
//
// sum is the SHA-256 of the keyfile as read back from destDir after
// writing, not of the source, so it describes the copy actually made. If
// the counter state is missing or corrupt, the backup gets a fresh counter
// from unusedCounterStart instead of 0 (which would make a restore reuse
// every nonce the key already used), and counterReset reports that.
func Export(destDir string, replace bool) (sum string, counterReset bool, err error) {
	dir, unlock, err := lockConfig()
	if err != nil {
		return "", false, err
	}
	defer unlock()

	key, err := LoadKey()
	if err != nil {
		return "", false, err
	}
	counter, cerr := readCounter(filepath.Join(dir, counterName))
	if cerr != nil {
		counterReset = true
		if counter, err = unusedCounterStart(); err != nil {
			return "", false, err
		}
	}

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		return "", false, err
	}
	destKey := filepath.Join(destDir, keyFileName)
	existing, err := os.ReadFile(destKey)
	switch {
	case err == nil:
		if !bytes.Equal(existing, key) && !replace {
			return "", false, fmt.Errorf("%w: %s", ErrBackupExists, destKey)
		}
	case !os.IsNotExist(err):
		return "", false, err
	}

	if err := fsutil.WriteFileAtomic(destKey, key, 0o600); err != nil {
		return "", false, err
	}
	if err := writeCounter(filepath.Join(destDir, counterName), counter); err != nil {
		return "", false, err
	}

	written, err := os.ReadFile(destKey)
	if err != nil {
		return "", false, err
	}
	if !bytes.Equal(written, key) {
		return "", false, fmt.Errorf("the keyfile copy at %s does not match the original", destKey)
	}
	s := sha256.Sum256(written)
	return hex.EncodeToString(s[:]), counterReset, nil
}

// Import installs a keyfile+counter backup from srcDir into the config dir.
//
// The backup is checked before anything changes: a keyfile of the right
// size and a readable counter, from a directory that isn't the config dir
// itself. A wrong or incomplete folder is refused with the active key and
// counter untouched.
//
// Whatever different key was active is preserved first as keyfile.old (with
// its own counter as counter.old, so the two can be restored together as a
// matched, safe-to-resume pair) — importing the wrong backup by mistake is
// then recoverable, not permanent. replaced reports whether that happened;
// re-importing the active key's own backup leaves keyfile.old alone.
//
// The counter becomes the higher of the backup's and the current one, never
// lower, the same high-water-mark rule as Keygen and Rotation.Commit — so a
// crash between writing the counter and the key can't leave either key with
// a counter below its real usage. Callers should still confirm with the
// user first when a keyfile already exists.
func Import(srcDir string) (replaced bool, err error) {
	srcKey := filepath.Join(srcDir, keyFileName)
	key, err := os.ReadFile(srcKey)
	if err != nil {
		return false, fmt.Errorf("no keyfile backup found: %w", err)
	}
	if len(key) != KeySize {
		return false, fmt.Errorf("%s is not an mlp keyfile (wrong size)", srcKey)
	}
	counter, err := readCounter(filepath.Join(srcDir, counterName))
	if err != nil {
		return false, fmt.Errorf("the backup in %s has no usable counter, nothing was changed (export it again with 'mlp keyfile export'): %w", srcDir, err)
	}

	dir, unlock, err := lockConfig()
	if err != nil {
		return false, err
	}
	defer unlock()

	if same, err := sameDir(srcDir, dir); err != nil {
		return false, err
	} else if same {
		return false, fmt.Errorf("%s is the config directory itself, not a backup", srcDir)
	}

	kp := filepath.Join(dir, keyFileName)
	cp := filepath.Join(dir, counterName)
	active, err := os.ReadFile(kp)
	switch {
	case err == nil:
		if !bytes.Equal(active, key) {
			if err := backupIfExists(kp, filepath.Join(dir, oldKeyFileName)); err != nil {
				return false, err
			}
			if err := backupIfExists(cp, filepath.Join(dir, oldCounterFileName)); err != nil {
				return false, err
			}
			replaced = true
		}
	case !os.IsNotExist(err):
		return false, err
	}

	if cur, err := readCounter(cp); err == nil {
		counter = max(counter, cur)
	}
	if err := writeCounter(cp, counter); err != nil {
		return false, err
	}
	if err := fsutil.WriteFileAtomic(kp, key, 0o600); err != nil {
		return false, err
	}
	return replaced, nil
}

func sameDir(a, b string) (bool, error) {
	sa, err := os.Stat(a)
	if err != nil {
		return false, err
	}
	sb, err := os.Stat(b)
	if err != nil {
		return false, err
	}
	return os.SameFile(sa, sb), nil
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

// writeCounter replaces the counter atomically: a crash mid-write leaves the
// previous value, never an empty or truncated file.
func writeCounter(path string, v uint64) error {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, v)
	return fsutil.WriteFileAtomic(path, buf, 0o600)
}

// Rotation is a new key being prepared in memory. Nothing touches disk
// until Commit, so a rotation that fails while re-encrypting files leaves
// the active key and counter untouched.
type Rotation struct {
	from []byte // the key being rotated away from
	key  []byte
	used uint64
}

// BeginRotation generates a fresh random key, held in memory only, to
// replace current (the active key the caller decrypted its files with).
func BeginRotation(current []byte) (*Rotation, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return &Rotation{from: current, key: key}, nil
}

// Key returns the new key being prepared.
func (r *Rotation) Key() []byte { return r.key }

// NextNonce returns the next counter-based nonce under the new key.
func (r *Rotation) NextNonce() [NonceSize]byte {
	r.used++
	return nonceFromCounter(r.used)
}

// Commit makes the new key active. The old key is kept as keyfile.old so
// files stranded by a crash mid-rotation stay recoverable. If the active
// key is no longer the one the rotation started from (replaced by another
// process meanwhile), Commit changes nothing and returns ErrKeyChanged.
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
	dir, unlock, err := lockConfig()
	if err != nil {
		return false, err
	}
	defer unlock()

	old, err := LoadKey()
	if err != nil {
		return false, err
	}
	if !bytes.Equal(old, r.from) {
		return false, ErrKeyChanged
	}
	kp := filepath.Join(dir, keyFileName)
	cp := filepath.Join(dir, counterName)

	cur, cerr := readCounter(cp)
	if cerr != nil {
		cur = 0
		fellBack = true
	}
	if err := writeCounter(cp, max(cur, r.used)); err != nil {
		return fellBack, err
	}
	if err := fsutil.WriteFileAtomic(filepath.Join(dir, oldKeyFileName), old, 0o600); err != nil {
		return fellBack, err
	}
	return fellBack, fsutil.WriteFileAtomic(kp, r.key, 0o600)
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

// SPDX-License-Identifier: GPL-3.0-or-later

package ops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/EldinBegano/mask-decryption/internal/crypto"
	"github.com/EldinBegano/mask-decryption/internal/fileformat"
	"github.com/EldinBegano/mask-decryption/internal/keystore"
)

// KeyFunc supplies the key. It runs only after cheap validation passes, so
// bad input never creates a keyfile as a side effect.
type KeyFunc func() ([]byte, error)

// Options tunes an operation.
type Options struct {
	// Force replaces an existing output file instead of failing. The
	// replacement is atomic: the new file is fully written beside the old one
	// and renamed over it, so a failure never destroys the existing file.
	Force bool
}

// Result describes one completed operation.
type Result struct {
	Input, Output string
	InSize        int  // bytes read (plaintext when encrypting)
	OutSize       int  // bytes written to the output payload
	Overwrote     bool // an existing output was replaced (Options.Force)
	NonceFallback bool // encrypt used a random nonce: counter state was lost

	// TimestampFailed is set on decrypt when the file was written
	// successfully but its original mtime/atime (stored in the .mlp
	// header) could not be applied. The file itself is not affected.
	TimestampFailed bool
}

// FileExt returns the extension without its dot. A name that is only a
// leading dot plus text (.env) is a dotfile with no extension.
func FileExt(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	if ext == base {
		return ""
	}
	return strings.TrimPrefix(ext, ".")
}

// DefaultEncryptPath replaces the file's extension with .mlp: notes.txt ->
// notes.mlp. Extensionless names get it appended: README -> README.mlp.
func DefaultEncryptPath(inputPath string) string {
	if FileExt(inputPath) == "" {
		return inputPath + ".mlp"
	}
	return strings.TrimSuffix(inputPath, filepath.Ext(inputPath)) + ".mlp"
}

// RestoreExt appends "."+ext to stem unless stem already carries that
// extension. Handles the normal case (encrypt replaced the extension with
// .mlp), the fallback name (notes.md.mlp, stem already ends in .md), and
// files renamed via an explicit output path.
func RestoreExt(stem, ext string) string {
	if ext == "" {
		return stem
	}
	if strings.HasSuffix(stem, "."+ext) {
		return stem
	}
	return stem + "." + ext
}

// DefaultDecryptPath is where a .mlp file with header extension ext decrypts to.
func DefaultDecryptPath(inputPath, ext string) string {
	return RestoreExt(strings.TrimSuffix(inputPath, ".mlp"), ext)
}

// ReadMLP parses the header and returns it, its own raw encoded bytes (for
// fileformat.Header.AAD — see there for why this matters), and the
// ciphertext that follows it.
func ReadMLP(path string) (hdr fileformat.Header, headerBytes, ciphertext []byte, err error) {
	f, err := os.Open(path)
	if err != nil {
		return fileformat.Header{}, nil, nil, err
	}
	defer f.Close()

	hdr, headerBytes, err = fileformat.ReadHeaderCapture(f)
	if err != nil {
		return hdr, headerBytes, nil, fmt.Errorf("%s: %w", path, err)
	}
	ciphertext, err = io.ReadAll(f)
	if err != nil {
		return hdr, headerBytes, nil, err
	}
	return hdr, headerBytes, ciphertext, nil
}

// EncryptFile encrypts inputPath to outputPath ("" means DefaultEncryptPath).
func EncryptFile(getKey KeyFunc, inputPath, outputPath string, opts Options) (Result, error) {
	res := Result{Input: inputPath}

	if strings.HasSuffix(inputPath, ".mlp") {
		return res, wrap(KindWrongType, fmt.Errorf("%s is already a .mlp file", inputPath))
	}

	// os.Stat follows symlinks, so a symlink input operates on its target.
	info, err := os.Stat(inputPath)
	if err != nil {
		return res, wrap(KindOther, err)
	}

	if outputPath == "" {
		outputPath = DefaultEncryptPath(inputPath)
	}
	res.Output = outputPath

	overwrite, err := checkOutput(info, outputPath, opts.Force)
	if err != nil {
		return res, err
	}
	res.Overwrote = overwrite

	key, err := getKey()
	if err != nil {
		return res, wrap(KindNoKey, err)
	}

	plaintext, err := os.ReadFile(inputPath)
	if err != nil {
		return res, wrap(KindOther, err)
	}

	// Compression happens here, before encryption: AES-GCM ciphertext is
	// high-entropy and doesn't compress at all, so it has to be the
	// plaintext. Kept only if it actually helped (already-compressed
	// input like jpg/mp4/zip usually doesn't shrink).
	packed, compressed, err := maybeCompress(plaintext)
	if err != nil {
		return res, wrap(KindOther, err)
	}

	nonce, fellBack, err := keystore.NextNonce()
	if err != nil {
		return res, wrap(KindOther, err)
	}
	res.NonceFallback = fellBack

	hdr := fileformat.Header{
		Ext:        FileExt(inputPath),
		Nonce:      nonce,
		ModTime:    info.ModTime(),
		AccessTime: accessTime(info),
		Compressed: compressed,
	}
	// Header bytes are computed once, here, and used both as what binds the
	// ciphertext to this exact header (GCM additional authenticated data —
	// see fileformat.Header.AAD) and as what actually gets written to disk.
	// Encoding the header and encrypting are always both "the current
	// version" going forward, so this is the header's own bytes directly,
	// not the version-conditional Header.AAD (that's only needed on the
	// decrypt side, which has to handle older files too).
	headerBytes, err := fileformat.EncodeHeader(hdr)
	if err != nil {
		return res, wrap(KindOther, err)
	}

	ciphertext, err := crypto.Encrypt(key, nonce[:], packed, headerBytes)
	if err != nil {
		return res, wrap(KindOther, err)
	}

	err = writeFile(outputPath, info.Mode().Perm(), overwrite, func(w io.Writer) error {
		if _, err := w.Write(headerBytes); err != nil {
			return err
		}
		_, err := w.Write(ciphertext)
		return err
	})
	if err != nil {
		return res, err
	}

	res.InSize, res.OutSize = len(plaintext), len(ciphertext)
	return res, nil
}

// DecryptFile decrypts the .mlp file at inputPath to outputPath ("" means the
// original name). The key is fetched after the header parses, so a malformed
// file reports as malformed rather than as a missing keyfile.
func DecryptFile(getKey KeyFunc, inputPath, outputPath string, opts Options) (Result, error) {
	res := Result{Input: inputPath}

	if !strings.HasSuffix(inputPath, ".mlp") {
		return res, wrap(KindWrongType, fmt.Errorf("%s is not a .mlp file", inputPath))
	}

	info, err := os.Stat(inputPath)
	if err != nil {
		return res, wrap(KindOther, err)
	}

	hdr, headerBytes, ciphertext, err := ReadMLP(inputPath)
	if err != nil {
		return res, wrap(KindOther, err)
	}

	key, err := getKey()
	if err != nil {
		return res, wrap(KindNoKey, err)
	}

	plaintext, err := crypto.Decrypt(key, hdr.Nonce[:], ciphertext, hdr.AAD(headerBytes))
	if err != nil {
		return res, wrap(KindAuth, fmt.Errorf("%s: %w", inputPath, err))
	}
	if hdr.Compressed {
		plaintext, err = decompress(plaintext)
		if err != nil {
			return res, wrap(KindOther, fmt.Errorf("%s: %w", inputPath, err))
		}
	}

	if outputPath == "" {
		outputPath = DefaultDecryptPath(inputPath, hdr.Ext)
	}
	res.Output = outputPath

	overwrite, err := checkOutput(info, outputPath, opts.Force)
	if err != nil {
		return res, err
	}
	res.Overwrote = overwrite

	err = writeFile(outputPath, info.Mode().Perm(), overwrite, func(w io.Writer) error {
		_, err := w.Write(plaintext)
		return err
	})
	if err != nil {
		return res, err
	}

	if hdr.HasTimestamps() {
		if err := os.Chtimes(outputPath, hdr.AccessTime, hdr.ModTime); err != nil {
			res.TimestampFailed = true
		}
	}

	res.InSize, res.OutSize = len(ciphertext), len(plaintext)
	return res, nil
}

// checkOutput decides whether writing to outputPath may proceed. It reports
// whether an existing file will be replaced. Writing a file onto itself or
// onto a directory is refused even with force.
func checkOutput(input os.FileInfo, outputPath string, force bool) (overwrite bool, err error) {
	st, err := os.Stat(outputPath)
	if err != nil {
		return false, nil
	}
	if os.SameFile(input, st) {
		return false, wrap(KindOther, fmt.Errorf("input and output are the same file: %s", outputPath))
	}
	if st.IsDir() {
		return false, wrap(KindOther, fmt.Errorf("output path %s is a directory", outputPath))
	}
	if !force {
		return false, wrap(KindExists, fmt.Errorf("output file %s already exists", outputPath))
	}
	return true, nil
}

// writeFile writes path via fill. Without overwrite it creates the file
// exclusively (an existing file is KindExists). With overwrite it writes a
// temp file beside path and renames it over path, so an existing file
// survives any failure. A partial file is never left behind.
func writeFile(path string, perm os.FileMode, overwrite bool, fill func(io.Writer) error) error {
	var f *os.File
	var err error
	if overwrite {
		f, err = os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	} else {
		f, err = os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	}
	if err != nil {
		if os.IsExist(err) {
			return wrap(KindExists, fmt.Errorf("output file %s already exists", path))
		}
		return wrap(KindOther, err)
	}
	tmp := f.Name()

	fail := func(err error) error {
		f.Close()
		os.Remove(tmp)
		return wrap(KindOther, err)
	}

	if err := fill(f); err != nil {
		return fail(err)
	}
	if err := f.Chmod(perm); err != nil {
		return fail(err)
	}
	if overwrite {
		if err := f.Sync(); err != nil {
			return fail(err)
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return wrap(KindOther, err)
	}
	if overwrite {
		if err := os.Rename(tmp, path); err != nil {
			os.Remove(tmp)
			return wrap(KindOther, err)
		}
	}
	return nil
}

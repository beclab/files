package reader

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// zipReader uses stdlib archive/zip for unencrypted zips. Encrypted
// entries are detected lazily and return an error so the handler can
// retry via the sevenz fallback path.
type zipReader struct {
	r *zip.ReadCloser
}

func openZip(absPath, _ string) (Reader, error) {
	r, err := zip.OpenReader(absPath)
	if err != nil {
		return nil, err
	}
	return &zipReader{r: r}, nil
}

// decodeZipName converts a non-UTF8 entry name to UTF-8. Older Windows
// zips encode CJK filenames in CP936/GBK; we try GBK as a single
// fallback. If decoding fails the raw bytes are returned (still
// printable, just garbled, which matches what most other tools do).
func decodeZipName(f *zip.File) string {
	// Bit 11 of the general purpose flag = name/comment are UTF-8.
	utf8 := f.Flags&0x800 != 0
	if utf8 {
		return f.Name
	}
	dec := simplifiedchinese.GBK.NewDecoder()
	if out, err := dec.String(f.Name); err == nil {
		return out
	}
	return f.Name
}

// zipEntryEncrypted reports whether the central-directory flag bit 0
// (encryption) is set. stdlib archive/zip does not expose IsEncrypted,
// so we check the flag directly.
func zipEntryEncrypted(f *zip.File) bool {
	return f.Flags&0x1 != 0
}

func (z *zipReader) Walk(ctx context.Context, fn func(Entry) error) error {
	for _, f := range z.r.File {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		name := decodeZipName(f)
		e := Entry{
			Path:      name,
			Size:      int64(f.UncompressedSize64),
			Modified:  f.Modified.Unix(),
			IsDir:     strings.HasSuffix(name, "/") || f.Mode().IsDir(),
			Encrypted: zipEntryEncrypted(f),
		}
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

// ErrEncryptedEntry is returned when Open is asked for an encrypted
// entry through the stdlib zip path. The handler catches this and
// retries via the sevenz fallback.
var ErrEncryptedEntry = errors.New("archive_entry_encrypted")

func (z *zipReader) Open(innerPath string) (io.ReadCloser, error) {
	for _, f := range z.r.File {
		if decodeZipName(f) != innerPath {
			continue
		}
		if zipEntryEncrypted(f) {
			return nil, ErrEncryptedEntry
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		return rc, nil
	}
	return nil, errors.New("entry not found: " + innerPath)
}

func (z *zipReader) Close() error { return z.r.Close() }

// Package reader exposes a streaming read-only view into archives for
// the /api/archive/.../entries (list) and /api/archive/.../entry
// (single-file stream) endpoints.
//
// Two implementations are routed by the archive's filename suffix:
//
//   - zip / tar / tar.gz / tgz  -> pure-stdlib readers in this package.
//     They avoid spawning a 7z subprocess per HTTP request, which is the
//     hot path for preview UIs.
//
//   - 7z / tar.bz2 / tar.xz / encrypted / multi-volume -> delegated to
//     pkg/archive/sevenz at one process per request.
//
// The interface is intentionally narrow:
//
//	Walk(ctx, fn)   - emit entries as they are discovered; fn may return
//	                  an error to abort early (used for client disconnect).
//	Open(innerPath) - return an io.ReadCloser for one entry's bytes.
//	Close()         - release any open file handle.
package reader

import (
	"context"
	"errors"
	"io"
	"strings"

	"files/pkg/common"
)

// Entry is one archived item surfaced to the HTTP layer.
type Entry struct {
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Modified  int64  `json:"modified"` // unix seconds; 0 when unknown
	IsDir     bool   `json:"is_dir"`
	Encrypted bool   `json:"encrypted"`
}

// Reader is the read-only archive interface returned by Open. Each
// instance owns at most one open file handle / one 7z subprocess (for
// the sevenz fallback) and MUST be Close()d by the caller.
type Reader interface {
	Walk(ctx context.Context, fn func(Entry) error) error
	Open(innerPath string) (io.ReadCloser, error)
	Close() error
}

// Open routes by the archive's filename suffix. password is forwarded
// only to sevenz-backed readers; stdlib zip cannot read AES-encrypted
// entries so they degrade to sevenz.
func Open(absPath, password string) (Reader, error) {
	if absPath == "" {
		return nil, errors.New("archive path is empty")
	}
	lower := strings.ToLower(absPath)

	// .001 means multi-volume; force sevenz fallback regardless of the
	// inner type because stdlib readers can't span volumes.
	if strings.HasSuffix(lower, ".001") {
		return newSevenz(absPath, password), nil
	}

	// stdlib paths
	if strings.HasSuffix(lower, ".zip") {
		// Encrypted zips fall back to sevenz; we can't tell at Open
		// time without reading the central directory, so the zip reader
		// itself detects per-entry encryption and lazily transitions
		// (see openZip).
		if password != "" {
			return newSevenz(absPath, password), nil
		}
		return openZip(absPath, password)
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return openTarGz(absPath)
	}
	if strings.HasSuffix(lower, ".tar") {
		return openTar(absPath)
	}

	// Everything else (7z / tar.bz2 / tar.xz / gzip-with-non-tar /
	// bzip2 / xz / unknown) goes through 7z.
	if common.ArchiveFormatFromName(absPath) == "" {
		return nil, errors.New("unsupported archive format: " + absPath)
	}
	return newSevenz(absPath, password), nil
}

package reader

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// tarReader and tarGzReader share most of the implementation; .tar
// reads directly from the file (random-access via Seek), .tar.gz must
// re-decompress from start for each Open call (gzip is not seekable).

// ----------------------------------------------------------------------
// .tar
// ----------------------------------------------------------------------

// tarReader opens an uncompressed tar archive. Walk reads sequentially;
// Open builds an in-memory index of (path -> byte offset in file) on
// first call so that subsequent seeks are O(1).
type tarReader struct {
	f     *os.File
	index map[string]tarEntryRef // nil until Walk completes or Open is first called
}

type tarEntryRef struct {
	offset int64
	size   int64
}

func openTar(absPath string) (Reader, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	return &tarReader{f: f}, nil
}

func (t *tarReader) Close() error { return t.f.Close() }

func (t *tarReader) Walk(ctx context.Context, fn func(Entry) error) error {
	if _, err := t.f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	tr := tar.NewReader(t.f)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		e := Entry{
			Path:     h.Name,
			Size:     h.Size,
			Modified: h.ModTime.Unix(),
			IsDir:    h.Typeflag == tar.TypeDir,
		}
		if err := fn(e); err != nil {
			return err
		}
	}
}

// ensureIndex builds the path -> offset map by walking once. Holds the
// file's seek position at end on success.
func (t *tarReader) ensureIndex() error {
	if t.index != nil {
		return nil
	}
	if _, err := t.f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	t.index = map[string]tarEntryRef{}
	tr := tar.NewReader(t.f)
	for {
		// archive/tar exposes the current data offset only indirectly;
		// we record the position BEFORE Next() returns by querying the
		// underlying file. Next() advances past the header, so the
		// file position when we're done reading h is exactly the start
		// of the entry data. This works because *os.File implements
		// io.Seeker.
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		off, err := t.f.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeDir {
			t.index[h.Name] = tarEntryRef{offset: off, size: h.Size}
		}
	}
}

func (t *tarReader) Open(innerPath string) (io.ReadCloser, error) {
	if err := t.ensureIndex(); err != nil {
		return nil, err
	}
	ref, ok := t.index[innerPath]
	if !ok {
		return nil, errors.New("entry not found: " + innerPath)
	}
	// We can't share the file handle with concurrent Open calls (they
	// would clobber seek position), so dup via a fresh os.Open.
	f, err := os.Open(t.f.Name())
	if err != nil {
		return nil, err
	}
	if _, err := f.Seek(ref.offset, io.SeekStart); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &limitedFileCloser{f: f, r: io.LimitReader(f, ref.size)}, nil
}

// limitedFileCloser wraps an io.LimitReader so Close releases the
// underlying file. Without this the file handle would leak.
type limitedFileCloser struct {
	f *os.File
	r io.Reader
}

func (l *limitedFileCloser) Read(p []byte) (int, error) { return l.r.Read(p) }
func (l *limitedFileCloser) Close() error               { return l.f.Close() }

// ----------------------------------------------------------------------
// .tar.gz / .tgz
// ----------------------------------------------------------------------

// tarGzReader provides Walk by streaming through gzip+tar; Open
// re-decompresses from the start to locate the entry. For entries
// larger than tarGzMaxOpenSize we refuse (a future revision could
// stream by index but cost/benefit is poor for small files which is
// the realistic preview case).
type tarGzReader struct {
	path string
}

// tarGzMaxOpenSize caps single-entry Open in tar.gz to keep memory
// pressure bounded for incidental previews. Larger entries return
// ErrEntryTooLarge so the FE knows to suggest "extract first".
const tarGzMaxOpenSize = 50 * 1024 * 1024

// ErrEntryTooLarge is returned by Open when the requested entry exceeds
// the in-memory cap for non-seekable archives.
var ErrEntryTooLarge = errors.New("archive_entry_too_large")

func openTarGz(absPath string) (Reader, error) {
	// Validate openability eagerly so callers fail fast.
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("gzip header: %w", err)
	}
	_ = gz.Close()
	_ = f.Close()
	return &tarGzReader{path: absPath}, nil
}

func (g *tarGzReader) Close() error { return nil }

func (g *tarGzReader) Walk(ctx context.Context, fn func(Entry) error) error {
	f, err := os.Open(g.path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		e := Entry{
			Path:     h.Name,
			Size:     h.Size,
			Modified: h.ModTime.Unix(),
			IsDir:    h.Typeflag == tar.TypeDir,
		}
		if err := fn(e); err != nil {
			return err
		}
	}
}

func (g *tarGzReader) Open(innerPath string) (io.ReadCloser, error) {
	f, err := os.Open(g.path)
	if err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			_ = gz.Close()
			_ = f.Close()
			return nil, errors.New("entry not found: " + innerPath)
		}
		if err != nil {
			_ = gz.Close()
			_ = f.Close()
			return nil, err
		}
		if h.Name != innerPath {
			continue
		}
		if h.Size > tarGzMaxOpenSize {
			_ = gz.Close()
			_ = f.Close()
			return nil, ErrEntryTooLarge
		}
		buf := &bytes.Buffer{}
		buf.Grow(int(h.Size))
		if _, err := io.Copy(buf, tr); err != nil {
			_ = gz.Close()
			_ = f.Close()
			return nil, err
		}
		_ = gz.Close()
		_ = f.Close()
		return io.NopCloser(buf), nil
	}
}

package reader

import (
	"context"
	"io"

	"files/pkg/archive/sevenz"
)

// sevenzReader is the fallback that delegates to the 7z CLI for formats
// we can't (or shouldn't) read with stdlib: 7z / tar.bz2 / tar.xz /
// encrypted zip / multi-volume archives.
//
// Each Walk and each Open spawns a fresh 7z process; processes are
// killed on ctx cancel or rc.Close (see sevenz.Walk and sevenz.Stream).
type sevenzReader struct {
	src      string
	password string
}

func newSevenz(absPath, password string) Reader {
	return &sevenzReader{src: absPath, password: password}
}

func (s *sevenzReader) Walk(ctx context.Context, fn func(Entry) error) error {
	return sevenz.Walk(ctx, sevenz.ListOpts{Src: s.src, Password: s.password}, func(e sevenz.Entry) error {
		out := Entry{
			Path:      e.Path,
			Size:      e.Size,
			IsDir:     e.IsDir,
			Encrypted: e.Encrypted,
		}
		if !e.Modified.IsZero() {
			out.Modified = e.Modified.Unix()
		}
		return fn(out)
	})
}

func (s *sevenzReader) Open(innerPath string) (io.ReadCloser, error) {
	// Stream creates a fresh subprocess; ctx is request-scoped at the
	// HTTP layer where this is called.
	return sevenz.Stream(context.Background(), sevenz.ListOpts{Src: s.src, Password: s.password}, innerPath)
}

func (s *sevenzReader) Close() error { return nil }

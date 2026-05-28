//go:build with_p7zip

// These tests spawn real 7z processes. Run with:
//   go test -tags with_p7zip ./pkg/archive/sevenz/...
// They are skipped in the default CI matrix and only execute in
// container images that have p7zip-full installed.
package sevenz

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"files/pkg/common"
)

// withTempDir gives the test a scratch dir that is cleaned up on exit.
func withTempDir(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	return d
}

func writeFile(t *testing.T, p string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompressExtractRoundtripZip(t *testing.T) {
	src := withTempDir(t)
	dst := withTempDir(t)
	writeFile(t, filepath.Join(src, "a.txt"), "hello")
	writeFile(t, filepath.Join(src, "sub/b.txt"), "world")

	arc := filepath.Join(dst, "out.zip")
	if err := Compress(context.Background(), CompressOpts{
		Dst:     arc,
		Sources: []string{src + "/."},
		Workdir: src,
		Format:  common.ArchiveFormatZip,
		Level:   5,
	}, nil); err != nil {
		t.Fatalf("compress: %v", err)
	}

	out := withTempDir(t)
	if err := Extract(context.Background(), ExtractOpts{
		Src:       arc,
		Dst:       out,
		Overwrite: common.ArchiveConflictOverwrite,
	}, nil); err != nil {
		t.Fatalf("extract: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(out, "a.txt"))
	if err != nil || string(body) != "hello" {
		t.Fatalf("a.txt readback: %v %q", err, body)
	}
	body, err = os.ReadFile(filepath.Join(out, "sub", "b.txt"))
	if err != nil || string(body) != "world" {
		t.Fatalf("sub/b.txt readback: %v %q", err, body)
	}
}

func TestCompressExtractEncrypted7z(t *testing.T) {
	src := withTempDir(t)
	dst := withTempDir(t)
	writeFile(t, filepath.Join(src, "secret.txt"), "top secret")

	arc := filepath.Join(dst, "out.7z")
	if err := Compress(context.Background(), CompressOpts{
		Dst:           arc,
		Sources:       []string{filepath.Join(src, "secret.txt")},
		Format:        common.ArchiveFormat7z,
		Level:         5,
		Password:      "pw1234",
		HeaderEncrypt: true,
	}, nil); err != nil {
		t.Fatalf("compress: %v", err)
	}

	// Wrong password should map to ErrPasswordInvalid.
	out1 := withTempDir(t)
	err := Extract(context.Background(), ExtractOpts{
		Src:       arc,
		Dst:       out1,
		Password:  "wrong",
		Overwrite: common.ArchiveConflictOverwrite,
	}, nil)
	if !errors.Is(err, ErrPasswordInvalid) {
		t.Fatalf("expected ErrPasswordInvalid, got %v", err)
	}

	// Correct password works.
	out2 := withTempDir(t)
	if err := Extract(context.Background(), ExtractOpts{
		Src:       arc,
		Dst:       out2,
		Password:  "pw1234",
		Overwrite: common.ArchiveConflictOverwrite,
	}, nil); err != nil {
		t.Fatalf("extract: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(out2, "secret.txt"))
	if err != nil || string(body) != "top secret" {
		t.Fatalf("secret.txt readback: %v %q", err, body)
	}
}

func TestWalkStreamCancel(t *testing.T) {
	src := withTempDir(t)
	dst := withTempDir(t)
	// Build a small zip with multiple entries.
	for i := 0; i < 20; i++ {
		writeFile(t, filepath.Join(src, "f", "x", "name"+string(rune('a'+i))+".txt"), "data")
	}
	arc := filepath.Join(dst, "out.zip")
	if err := Compress(context.Background(), CompressOpts{
		Dst: arc, Sources: []string{src}, Format: common.ArchiveFormatZip, Level: 5,
	}, nil); err != nil {
		t.Fatalf("compress: %v", err)
	}

	// Walk should emit entries until fn returns an error, then bail
	// quickly without leaking the subprocess.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var seen int64
	walkErr := Walk(ctx, ListOpts{Src: arc}, func(e Entry) error {
		if atomic.AddInt64(&seen, 1) == 3 {
			return errors.New("client closed")
		}
		return nil
	})
	if walkErr == nil {
		t.Fatalf("expected walk error from fn, got nil")
	}
	if seen < 3 {
		t.Fatalf("expected to see at least 3 entries before cancel, saw %d", seen)
	}
}

func TestStreamSingleEntry(t *testing.T) {
	src := withTempDir(t)
	dst := withTempDir(t)
	writeFile(t, filepath.Join(src, "hello.txt"), "hello world\n")
	arc := filepath.Join(dst, "out.zip")
	if err := Compress(context.Background(), CompressOpts{
		Dst:     arc,
		Sources: []string{filepath.Join(src, "hello.txt")},
		Format:  common.ArchiveFormatZip,
		Level:   5,
	}, nil); err != nil {
		t.Fatalf("compress: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rc, err := Stream(ctx, ListOpts{Src: arc}, "hello.txt")
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer rc.Close()
	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, rc); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if buf.String() != "hello world\n" {
		t.Fatalf("unexpected body: %q", buf.String())
	}
}

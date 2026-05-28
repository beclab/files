package reader

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// buildZip creates a zip at p with the given files (name -> content).
// If name ends with "/" the entry is a directory.
func buildZip(t *testing.T, p string, files map[string]string) {
	t.Helper()
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, content := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()}
		if strings.HasSuffix(name, "/") {
			h.SetMode(0o755 | os.ModeDir)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(name, "/") {
			if _, err := w.Write([]byte(content)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestZipWalkAndOpen(t *testing.T) {
	tmp := t.TempDir()
	arc := filepath.Join(tmp, "test.zip")
	buildZip(t, arc, map[string]string{
		"a.txt":     "hello",
		"sub/b.txt": "world",
		"empty/":    "",
	})

	r, err := Open(arc, "")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()

	got := map[string]Entry{}
	if err := r.Walk(context.Background(), func(e Entry) error {
		got[e.Path] = e
		return nil
	}); err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if _, ok := got["a.txt"]; !ok {
		t.Errorf("a.txt missing in walk: %v", got)
	}
	if _, ok := got["sub/b.txt"]; !ok {
		t.Errorf("sub/b.txt missing in walk: %v", got)
	}
	if e := got["empty/"]; !e.IsDir {
		t.Errorf("empty/ should be dir, got %+v", e)
	}

	rc, err := r.Open("sub/b.txt")
	if err != nil {
		t.Fatalf("Open entry: %v", err)
	}
	defer rc.Close()
	body, err := io.ReadAll(rc)
	if err != nil || string(body) != "world" {
		t.Errorf("body=%q err=%v", body, err)
	}
}

func TestZipWalkCancel(t *testing.T) {
	tmp := t.TempDir()
	arc := filepath.Join(tmp, "many.zip")
	files := map[string]string{}
	for i := 0; i < 100; i++ {
		files["f"+string(rune('a'+i%26))+string(rune('0'+i/10))+".txt"] = "x"
	}
	buildZip(t, arc, files)

	r, err := Open(arc, "")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	ctx, cancel := context.WithCancel(context.Background())
	count := 0
	err = r.Walk(ctx, func(e Entry) error {
		count++
		if count == 5 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestZipFnReturnError(t *testing.T) {
	tmp := t.TempDir()
	arc := filepath.Join(tmp, "x.zip")
	buildZip(t, arc, map[string]string{"a": "1", "b": "2", "c": "3"})

	r, _ := Open(arc, "")
	defer r.Close()
	sentinel := errors.New("client closed")
	err := r.Walk(context.Background(), func(e Entry) error {
		if e.Path == "b" {
			return sentinel
		}
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error, got %v", err)
	}
}

// buildTar writes an uncompressed tar at p with the given files.
func buildTar(t *testing.T, p string, files map[string]string) {
	t.Helper()
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tw := tar.NewWriter(f)
	for name, body := range files {
		h := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), ModTime: time.Now()}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTarWalkAndOpen(t *testing.T) {
	tmp := t.TempDir()
	arc := filepath.Join(tmp, "t.tar")
	buildTar(t, arc, map[string]string{"a": "one", "b": "two"})
	r, err := Open(arc, "")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got := map[string]int64{}
	r.Walk(context.Background(), func(e Entry) error {
		got[e.Path] = e.Size
		return nil
	})
	if got["a"] != 3 || got["b"] != 3 {
		t.Errorf("unexpected walk: %v", got)
	}
	rc, err := r.Open("b")
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	body, _ := io.ReadAll(rc)
	if string(body) != "two" {
		t.Errorf("body = %q", body)
	}
	// not-found
	if _, err := r.Open("nope"); err == nil {
		t.Errorf("expected error for missing entry")
	}
}

// buildTarGz writes a .tar.gz at p.
func buildTarGz(t *testing.T, p string, files map[string]string) {
	t.Helper()
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for name, body := range files {
		h := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), ModTime: time.Now()}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTarGzWalkAndOpen(t *testing.T) {
	tmp := t.TempDir()
	arc := filepath.Join(tmp, "t.tar.gz")
	buildTarGz(t, arc, map[string]string{"hello": "world"})
	r, err := Open(arc, "")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	count := 0
	r.Walk(context.Background(), func(e Entry) error { count++; return nil })
	if count != 1 {
		t.Errorf("expected 1 entry, got %d", count)
	}
	rc, _ := r.Open("hello")
	defer rc.Close()
	body, _ := io.ReadAll(rc)
	if string(body) != "world" {
		t.Errorf("body=%q", body)
	}
}

func TestTarGzOpenTooLarge(t *testing.T) {
	tmp := t.TempDir()
	arc := filepath.Join(tmp, "big.tar.gz")
	big := strings.Repeat("x", tarGzMaxOpenSize+1)
	buildTarGz(t, arc, map[string]string{"big": big})
	r, _ := Open(arc, "")
	defer r.Close()
	_, err := r.Open("big")
	if !errors.Is(err, ErrEntryTooLarge) {
		t.Errorf("expected ErrEntryTooLarge, got %v", err)
	}
}

func TestOpenUnsupportedFormat(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "foo.unknown")
	os.WriteFile(p, []byte("?"), 0o644)
	_, err := Open(p, "")
	if err == nil {
		t.Errorf("expected error for unknown format")
	}
}

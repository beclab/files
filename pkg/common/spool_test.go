package common

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSpoolWriter_StaysInMemoryBelowLimit(t *testing.T) {
	s := NewSpoolWriter(64)
	defer s.Cleanup()

	payload := []byte("hello world")
	if _, err := s.Write(payload); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if s.SpilledToDisk() {
		t.Fatal("expected spool to stay in memory below limit")
	}
	if s.Size() != int64(len(payload)) {
		t.Fatalf("Size = %d, want %d", s.Size(), len(payload))
	}

	r, err := s.Reader()
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("Reader returned %q, want %q", got, payload)
	}
}

func TestSpoolWriter_SpillsAcrossThreshold(t *testing.T) {
	const limit = 16
	s := NewSpoolWriter(limit)
	defer s.Cleanup()

	first := bytes.Repeat([]byte("a"), 10)
	if _, err := s.Write(first); err != nil {
		t.Fatalf("Write first: %v", err)
	}
	if s.SpilledToDisk() {
		t.Fatal("did not expect spill before threshold")
	}

	second := bytes.Repeat([]byte("b"), 20)
	if _, err := s.Write(second); err != nil {
		t.Fatalf("Write second: %v", err)
	}
	if !s.SpilledToDisk() {
		t.Fatal("expected spill after exceeding threshold")
	}
	if got, want := s.Size(), int64(len(first)+len(second)); got != want {
		t.Fatalf("Size = %d, want %d", got, want)
	}

	r, err := s.Reader()
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	want := append(append([]byte{}, first...), second...)
	if !bytes.Equal(got, want) {
		t.Fatalf("Reader returned %q, want %q", got, want)
	}
}

func TestSpoolWriter_LargeSingleWriteSpills(t *testing.T) {
	s := NewSpoolWriter(8)
	defer s.Cleanup()

	payload := bytes.Repeat([]byte{'z'}, 1024)
	if _, err := s.Write(payload); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !s.SpilledToDisk() {
		t.Fatal("large single write should spill")
	}

	r, err := s.Reader()
	if err != nil {
		t.Fatalf("Reader: %v", err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %d bytes, want %d", len(got), len(payload))
	}
}

func TestSpoolWriter_CleanupRemovesTempFile(t *testing.T) {
	s := NewSpoolWriter(4)
	if _, err := s.Write([]byte(strings.Repeat("x", 256))); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !s.SpilledToDisk() {
		t.Fatal("expected spill")
	}
	tempName := s.file.Name()

	if err := s.Cleanup(); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if _, err := os.Stat(tempName); !os.IsNotExist(err) {
		t.Fatalf("temp file %q still exists after Cleanup: err=%v", tempName, err)
	}

	if err := s.Cleanup(); err != nil {
		t.Fatalf("second Cleanup: %v", err)
	}
}

func TestSpoolWriter_DefaultMemLimit(t *testing.T) {
	s := NewSpoolWriter(0)
	defer s.Cleanup()

	if s.memLimit != SpoolDefaultMemLimit {
		t.Fatalf("default memLimit = %d, want %d", s.memLimit, SpoolDefaultMemLimit)
	}

	s2 := NewSpoolWriter(-1)
	defer s2.Cleanup()
	if s2.memLimit != SpoolDefaultMemLimit {
		t.Fatalf("negative memLimit = %d, want %d", s2.memLimit, SpoolDefaultMemLimit)
	}
}

func TestSpoolWriter_TempFileLivesUnderTempDir(t *testing.T) {
	s := NewSpoolWriter(1)
	defer s.Cleanup()

	if _, err := s.Write([]byte("spill")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !s.SpilledToDisk() {
		t.Fatal("expected spill")
	}
	got := s.file.Name()
	want := os.TempDir()
	if filepath.Dir(got) != filepath.Clean(want) {
		t.Fatalf("temp file %q is not under TempDir %q", got, want)
	}
}

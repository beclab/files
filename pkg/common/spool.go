package common

import (
	"bytes"
	"io"
	"os"
)

// SpoolDefaultMemLimit is a conservative in-memory threshold for
// spooling rebuilt request bodies; anything past this spills to a
// temp file under os.TempDir(). The number is intentionally smaller
// than Hertz's max-body-size cap (20 MiB) so a single in-flight
// request stays well under the framework limit even with a copy.
const SpoolDefaultMemLimit = 4 << 20

// SpoolWriter buffers writes in memory up to memLimit bytes, then
// transparently spills the entire content (and subsequent writes) to
// a temporary file. Use Reader once, after all writes complete, to
// obtain a positioned-at-zero reader, and Cleanup to release any
// temp file that was created.
//
// SpoolWriter is NOT safe for concurrent use.
//
// The intended pattern:
//
//	spool := common.NewSpoolWriter(common.SpoolDefaultMemLimit)
//	defer spool.Cleanup()
//	// ... write into spool ...
//	r, err := spool.Reader()
//	// ... pass r to a downstream consumer; do not Close it ...
type SpoolWriter struct {
	memLimit int64
	buf      bytes.Buffer
	file     *os.File
	size     int64
}

// NewSpoolWriter constructs a SpoolWriter with the given in-memory
// threshold. A non-positive memLimit is replaced with
// SpoolDefaultMemLimit.
func NewSpoolWriter(memLimit int64) *SpoolWriter {
	if memLimit <= 0 {
		memLimit = SpoolDefaultMemLimit
	}
	return &SpoolWriter{memLimit: memLimit}
}

// Write appends p to the spool, spilling to disk if the total size
// would exceed memLimit. The returned count never exceeds len(p).
func (s *SpoolWriter) Write(p []byte) (int, error) {
	if s.file != nil {
		n, err := s.file.Write(p)
		s.size += int64(n)
		return n, err
	}
	if s.size+int64(len(p)) > s.memLimit {
		if err := s.spill(); err != nil {
			return 0, err
		}
		n, err := s.file.Write(p)
		s.size += int64(n)
		return n, err
	}
	n, err := s.buf.Write(p)
	s.size += int64(n)
	return n, err
}

func (s *SpoolWriter) spill() error {
	f, err := os.CreateTemp("", "files-spool-")
	if err != nil {
		return err
	}
	if _, err := f.Write(s.buf.Bytes()); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return err
	}
	s.buf.Reset()
	s.file = f
	return nil
}

// Size returns the total number of bytes written so far.
func (s *SpoolWriter) Size() int64 { return s.size }

// SpilledToDisk reports whether this spool has overflowed to a temp
// file. Useful for tests and observability.
func (s *SpoolWriter) SpilledToDisk() bool { return s.file != nil }

// Reader returns a Reader over all content written so far, positioned
// at byte zero. It must be called at most once. The returned Reader
// is owned by the SpoolWriter; callers must not Close it - call
// Cleanup when done.
func (s *SpoolWriter) Reader() (io.Reader, error) {
	if s.file != nil {
		if _, err := s.file.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		return readNoCloser{Reader: s.file}, nil
	}
	return bytes.NewReader(s.buf.Bytes()), nil
}

// Cleanup releases any resources held by the spool. It is safe to
// call multiple times and should typically be deferred immediately
// after construction.
func (s *SpoolWriter) Cleanup() error {
	if s.file == nil {
		return nil
	}
	name := s.file.Name()
	cerr := s.file.Close()
	rerr := os.Remove(name)
	s.file = nil
	if cerr != nil {
		return cerr
	}
	return rerr
}

// readNoCloser hides the Closer of an *os.File so a downstream
// consumer that opportunistically calls Close on its body reader
// cannot race with SpoolWriter.Cleanup.
type readNoCloser struct {
	io.Reader
}

func (readNoCloser) Close() error { return nil }

// Package sevenz wraps the p7zip-full `7z` CLI as the archive compress
// / extract / list / single-entry stream backend. We shell out (rather
// than link a Go-native library) because 7z handles every format we
// declared support for in one binary -- zip / 7z / tar / tar.gz / tar.bz2
// / tar.xz / gzip / bzip2 / xz -- plus AES-256 password encryption and
// real multi-volume archives, none of which are covered by stdlib.
//
// The runner is intentionally stateless: every call spawns a fresh 7z
// process bound to the caller's context.Context. Cancelling the context
// kills the subprocess, so a client disconnect on /entries or a task
// cancellation on /compress propagates to the running 7z immediately.
package sevenz

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"files/pkg/common"

	"k8s.io/klog/v2"
)

// Binary is the 7z executable name; we look it up via PATH at call
// time so that an unavailable binary surfaces a clean error instead of
// a panic at init.
const Binary = "7z"

// CompressOpts is the input to Compress.
type CompressOpts struct {
	// Dst is the absolute path of the archive to produce. For multi-
	// volume archives the on-disk files will be Dst+".001", ".002", ...
	Dst string
	// Sources is the list of absolute paths to archive. 7z preserves
	// the basename of each entry; pass entries relative to Workdir if
	// you want to control the in-archive prefix.
	Sources []string
	// Workdir is the cwd of the 7z process (controls relative paths in
	// the archive). Empty means inherit the calling process's cwd.
	Workdir string
	// Format must be one of common.ArchiveFormatsWrite.
	Format string
	// Level is 0..9; 0 means store (no compression). Caller should pre
	// normalize via models.ArchiveOption.NormalizeForCompress.
	Level int
	// Password, if non-empty, enables encryption.
	Password string
	// HeaderEncrypt corresponds to 7z's -mhe=on (7z format only). Set
	// by NormalizeForCompress when Format == "7z" && Password != "".
	HeaderEncrypt bool
	// VolumeSizeBytes >0 enables multi-volume; the on-disk filenames
	// will be Dst+".001", ".002", ...
	VolumeSizeBytes int64
	// PreserveSymlinks corresponds to 7z's -snl.
	PreserveSymlinks bool
}

// ExtractOpts is the input to Extract.
type ExtractOpts struct {
	// Src is the archive path. For multi-volume archives this is the
	// .001 file; 7z auto-detects the rest.
	Src string
	// Dst is the absolute target directory; created if missing.
	Dst string
	// Password, if non-empty, is sent on argv as -p<password>.
	Password string
	// PreserveSymlinks corresponds to 7z's -snl.
	PreserveSymlinks bool
	// Overwrite is one of common.ArchiveConflict* (rename / overwrite /
	// skip). Caller should pre-normalize.
	Overwrite string
}

// ListOpts is the input to Walk / Stream.
type ListOpts struct {
	Src      string
	Password string
}

// Entry is one entry inside an archive surfaced by Walk.
type Entry struct {
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Modified  time.Time `json:"modified"`
	IsDir     bool      `json:"is_dir"`
	Encrypted bool      `json:"encrypted"`
}

// ProgressFn is invoked from progress lines parsed off the 7z stdout
// stream. percent is 0..100; transferred is total bytes written so far
// (best-effort: 7z only reports percent reliably, transferred is
// computed by the caller as totalSize*percent/100 when totalSize is
// known).
type ProgressFn func(percent int, transferred int64)

// WalkFn is the per-entry callback for Walk. Returning a non-nil error
// causes Walk to kill the 7z subprocess and return that error.
type WalkFn func(Entry) error

// ----------------------------------------------------------------------
// Error classification
// ----------------------------------------------------------------------

// 7z exit codes:
//   0 - success
//   1 - non-fatal warnings (e.g., file in use)
//   2 - fatal error (most encryption / corruption cases)
//   7 - command-line usage error (we treat as a bug)
//   8 - not enough memory
// 255 - user-stopped
//
// We map a few well-known stderr patterns to typed errors so the HTTP
// layer can translate them to stable user-facing codes (e.g., 401 for
// password problems).

// ErrPasswordRequired is returned when 7z indicates the archive is
// encrypted but no password was supplied.
var ErrPasswordRequired = errors.New("archive_password_required")

// ErrPasswordInvalid is returned when the supplied password is wrong.
var ErrPasswordInvalid = errors.New("archive_password_invalid")

// ErrCorrupt is returned when 7z reports a malformed / unreadable
// archive.
var ErrCorrupt = errors.New("archive_corrupt")

// ErrVolumeMissing is returned when 7z cannot find a subsequent volume
// file (e.g., .002 missing while opening .001).
var ErrVolumeMissing = errors.New("archive_volume_missing")

// Classify inspects err and the captured stderr/stdout buffer of a 7z
// subprocess and maps to one of the typed errors above when possible.
func Classify(err error, out string) error {
	if err == nil {
		return nil
	}
	low := strings.ToLower(out)
	switch {
	case strings.Contains(low, "wrong password"),
		strings.Contains(low, "data error in encrypted file. wrong password?"),
		strings.Contains(low, "cannot open encrypted archive. wrong password?"):
		return ErrPasswordInvalid
	case strings.Contains(low, "enter password") || strings.Contains(low, "password is not defined"):
		return ErrPasswordRequired
	case strings.Contains(low, "can not find the file for archive volume"),
		strings.Contains(low, "missing volume"):
		return ErrVolumeMissing
	case strings.Contains(low, "is not archive"),
		strings.Contains(low, "headers error"),
		strings.Contains(low, "unexpected end of archive"):
		return ErrCorrupt
	}
	return err
}

// ----------------------------------------------------------------------
// Argument helpers
// ----------------------------------------------------------------------

// formatTypeFlag maps our internal format names to 7z's -t<x> tokens.
// gzip / bzip2 / xz are single-stream formats which 7z calls gzip /
// bzip2 / xz respectively when used as -t.
func formatTypeFlag(f string) (string, error) {
	switch f {
	case common.ArchiveFormatZip:
		return "zip", nil
	case common.ArchiveFormat7z:
		return "7z", nil
	case common.ArchiveFormatTar:
		return "tar", nil
	case common.ArchiveFormatTarGz, common.ArchiveFormatTgz, common.ArchiveFormatGzip:
		return "gzip", nil
	case common.ArchiveFormatTarBz2, common.ArchiveFormatBzip2:
		return "bzip2", nil
	case common.ArchiveFormatTarXz, common.ArchiveFormatXz:
		return "xz", nil
	}
	return "", fmt.Errorf("unsupported archive format: %s", f)
}

func overwriteFlag(c string) string {
	switch c {
	case common.ArchiveConflictOverwrite:
		return "-aoa"
	case common.ArchiveConflictSkip:
		return "-aos"
	default:
		// rename matches 7z's "auto-rename" behavior.
		return "-aou"
	}
}

// redactArgs returns a copy of argv with -p<pwd> tokens masked, for
// logging. We never log the original args directly.
func redactArgs(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		if strings.HasPrefix(a, "-p") && len(a) > 2 {
			out[i] = "-p***"
		} else {
			out[i] = a
		}
	}
	return out
}

// ----------------------------------------------------------------------
// Compress
// ----------------------------------------------------------------------

// Compress invokes `7z a` to build opts.Dst from opts.Sources. If
// opts.VolumeSizeBytes > 0 the on-disk filenames will be Dst+".NNN".
//
// The function blocks until 7z exits or ctx is cancelled.
func Compress(ctx context.Context, opts CompressOpts, prog ProgressFn) error {
	bin, err := exec.LookPath(Binary)
	if err != nil {
		return fmt.Errorf("7z binary not found in PATH: %w", err)
	}
	if opts.Dst == "" || len(opts.Sources) == 0 {
		return errors.New("sevenz.Compress: dst and at least one source required")
	}

	t, err := formatTypeFlag(opts.Format)
	if err != nil {
		return err
	}

	// For tar.gz / tar.bz2 / tar.xz we ask 7z to build a tar then pipe
	// it through the corresponding compressor. 7z supports this via a
	// single invocation: -t<inner>:<outer> isn't standard, so the
	// conventional workflow is `7z a -ttar -so` piped to `7z a -t<comp>
	// -si`. To keep things simple and avoid two subprocesses we use a
	// single `7z a -t<format>` where <format> is gzip/bzip2/xz and pass
	// the source as a pre-built tar file, OR we let 7z auto-derive.
	// Easiest path: for tar.* compose by detecting suffix and using
	// 7z's nested compression syntax `-tgzip:tar` etc. Actually 7z
	// natively understands these via -ttar and naming the dst foo.tar
	// then re-running -tgzip foo.tar foo.tar.gz. We avoid this two-pass
	// dance for now and only support tar.* via post-pipe: produce a
	// .tar first, then compress.
	if isCompoundTar(opts.Format) {
		return compressTarThenCompress(ctx, bin, opts, prog)
	}

	args := []string{"a", "-t" + t, fmt.Sprintf("-mx=%d", opts.Level)}
	if opts.Password != "" {
		args = append(args, "-p"+opts.Password)
		if opts.HeaderEncrypt && opts.Format == common.ArchiveFormat7z {
			args = append(args, "-mhe=on")
		}
	}
	if opts.VolumeSizeBytes > 0 {
		args = append(args, fmt.Sprintf("-v%db", opts.VolumeSizeBytes))
	}
	if opts.PreserveSymlinks {
		args = append(args, "-snl")
	}
	args = append(args, "-bsp1", "-bso1", "-bse2", "-bb0", "-y", "--", opts.Dst)
	args = append(args, opts.Sources...)

	return runWithProgress(ctx, bin, args, opts.Workdir, prog)
}

// isCompoundTar reports whether the format is a tar wrapped in a stream
// compressor (gz/bz2/xz). For these we have to produce the tar then
// compress; 7z's `a -ttar.gz` is not a thing.
func isCompoundTar(f string) bool {
	return f == common.ArchiveFormatTarGz ||
		f == common.ArchiveFormatTgz ||
		f == common.ArchiveFormatTarBz2 ||
		f == common.ArchiveFormatTarXz
}

// compressTarThenCompress builds a .tar in a temp file next to Dst, then
// runs 7z again to wrap it in gzip/bzip2/xz, writing to Dst. We delete
// the intermediate .tar on success and on failure (defer).
func compressTarThenCompress(ctx context.Context, bin string, opts CompressOpts, prog ProgressFn) (err error) {
	tarPath := opts.Dst + ".intermediate.tar"
	// Phase 1: build the tar. We weight it as 80% of overall progress.
	tarArgs := []string{"a", "-ttar", "-bsp1", "-bso1", "-bse2", "-bb0", "-y", "--", tarPath}
	tarArgs = append(tarArgs, opts.Sources...)

	wrapped := func(p int, t int64) {
		// Map 0..100% of tar phase to 0..80%.
		if prog != nil {
			prog(p*80/100, t)
		}
	}
	if err := runWithProgress(ctx, bin, tarArgs, opts.Workdir, wrapped); err != nil {
		_ = removeIfExists(tarPath)
		return err
	}

	defer func() {
		// Best-effort cleanup of the intermediate tar regardless of
		// outcome. removeIfExists is silent if the file is already gone.
		_ = removeIfExists(tarPath)
	}()

	// Phase 2: wrap with gzip/bzip2/xz, remaining 80..100%.
	var compType string
	switch opts.Format {
	case common.ArchiveFormatTarGz, common.ArchiveFormatTgz:
		compType = "gzip"
	case common.ArchiveFormatTarBz2:
		compType = "bzip2"
	case common.ArchiveFormatTarXz:
		compType = "xz"
	default:
		return fmt.Errorf("unexpected compound format: %s", opts.Format)
	}

	args := []string{"a", "-t" + compType, fmt.Sprintf("-mx=%d", opts.Level),
		"-bsp1", "-bso1", "-bse2", "-bb0", "-y", "--", opts.Dst, tarPath}

	wrapped2 := func(p int, t int64) {
		if prog != nil {
			prog(80+p*20/100, t)
		}
	}
	return runWithProgress(ctx, bin, args, opts.Workdir, wrapped2)
}

// ----------------------------------------------------------------------
// Extract
// ----------------------------------------------------------------------

// Extract invokes `7z x` to expand opts.Src into opts.Dst. For multi-
// volume archives pass the .001 path as Src.
func Extract(ctx context.Context, opts ExtractOpts, prog ProgressFn) error {
	bin, err := exec.LookPath(Binary)
	if err != nil {
		return fmt.Errorf("7z binary not found in PATH: %w", err)
	}
	if opts.Src == "" || opts.Dst == "" {
		return errors.New("sevenz.Extract: src and dst required")
	}

	args := []string{"x", "-o" + opts.Dst}
	if opts.Password != "" {
		args = append(args, "-p"+opts.Password)
	} else {
		// Force 7z to bail rather than block on a tty prompt when the
		// archive is encrypted and we have no password. -p (no value)
		// is reserved; using a dummy "" via -p"" makes 7z fail fast
		// with "Wrong password" which Classify maps to PasswordRequired.
		args = append(args, "-p")
	}
	if opts.PreserveSymlinks {
		args = append(args, "-snl")
	}
	args = append(args, overwriteFlag(opts.Overwrite),
		"-bsp1", "-bso1", "-bse2", "-bb0", "-y", "--", opts.Src)

	return runWithProgress(ctx, bin, args, "", prog)
}

// ----------------------------------------------------------------------
// Walk (streaming list)
// ----------------------------------------------------------------------

// Walk invokes `7z l -slt` and parses the structured-output blocks,
// emitting one Entry per archived item via fn. Returning a non-nil
// error from fn (e.g., on client disconnect) kills the subprocess
// promptly.
func Walk(ctx context.Context, opts ListOpts, fn WalkFn) error {
	bin, err := exec.LookPath(Binary)
	if err != nil {
		return fmt.Errorf("7z binary not found in PATH: %w", err)
	}
	if opts.Src == "" {
		return errors.New("sevenz.Walk: src required")
	}

	args := []string{"l", "-slt", "-bso1", "-bse2", "-bb0", "-y"}
	if opts.Password != "" {
		args = append(args, "-p"+opts.Password)
	} else {
		args = append(args, "-p")
	}
	args = append(args, "--", opts.Src)

	klog.V(4).Infof("[sevenz] Walk: %s %s", bin, strings.Join(redactArgs(args), " "))

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if startErr := cmd.Start(); startErr != nil {
		return fmt.Errorf("start 7z: %w", startErr)
	}

	// Collect stderr in full so Classify can map password / corrupt
	// errors. 7z's structured stderr is small (a few lines on error).
	var stderrBuf strings.Builder
	var stderrWg sync.WaitGroup
	stderrWg.Add(1)
	go func() {
		defer stderrWg.Done()
		_, _ = io.Copy(&stderrBuf, stderr)
	}()

	// Parser state machine: lines are key=value; entries are separated
	// by blank lines; the first separator line is "----------" after
	// which entries begin.
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	inEntries := false
	cur := map[string]string{}
	emit := func() error {
		if len(cur) == 0 {
			return nil
		}
		e, ok := parseEntry(cur)
		cur = map[string]string{}
		if !ok {
			return nil
		}
		return fn(e)
	}

	var walkErr error
	for scanner.Scan() {
		line := scanner.Text()
		if !inEntries {
			if strings.HasPrefix(line, "----------") {
				inEntries = true
			}
			continue
		}
		if line == "" {
			if err := emit(); err != nil {
				walkErr = err
				break
			}
			continue
		}
		k, v, ok := splitKV(line)
		if ok {
			cur[k] = v
		}
	}
	// Drain the final entry if no trailing blank line.
	if walkErr == nil && len(cur) > 0 {
		walkErr = emit()
	}
	if walkErr == nil {
		walkErr = scanner.Err()
	}

	if walkErr != nil {
		// Kill the subprocess so it doesn't keep writing into a closed
		// pipe; cmd.Wait below will then return promptly.
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}

	waitErr := cmd.Wait()
	stderrWg.Wait()

	// Prefer the walk error (caller-driven cancellation) over the wait
	// error (process killed/exited).
	if walkErr != nil {
		return walkErr
	}
	if waitErr != nil {
		return Classify(waitErr, stderrBuf.String())
	}
	return nil
}

// parseEntry converts a key/value map (one 7z -slt entry) into an Entry.
// Returns ok=false for header records (no Path).
func parseEntry(m map[string]string) (Entry, bool) {
	path, ok := m["Path"]
	if !ok || path == "" {
		return Entry{}, false
	}
	e := Entry{Path: path}
	if s, ok := m["Size"]; ok {
		e.Size, _ = strconv.ParseInt(s, 10, 64)
	}
	if v, ok := m["Folder"]; ok && (v == "+" || strings.EqualFold(v, "true")) {
		e.IsDir = true
	}
	if v, ok := m["Attributes"]; ok && strings.Contains(v, "D") {
		e.IsDir = true
	}
	if v, ok := m["Encrypted"]; ok && (v == "+" || strings.EqualFold(v, "true")) {
		e.Encrypted = true
	}
	if v, ok := m["Modified"]; ok && v != "" {
		// 7z prints "2024-01-02 15:04:05" (sometimes with fractional
		// seconds). Try a couple of layouts.
		for _, layout := range []string{
			"2006-01-02 15:04:05",
			"2006-01-02 15:04:05.000000000",
			"2006-01-02 15:04:05.000",
		} {
			if t, err := time.Parse(layout, v); err == nil {
				e.Modified = t
				break
			}
		}
	}
	return e, true
}

func splitKV(line string) (string, string, bool) {
	i := strings.Index(line, " = ")
	if i < 0 {
		return "", "", false
	}
	return line[:i], line[i+3:], true
}

// ----------------------------------------------------------------------
// Stream (extract one entry to stdout)
// ----------------------------------------------------------------------

// Stream invokes `7z e -so -- src inner` and returns the subprocess's
// stdout as an io.ReadCloser. Closing the returned reader kills the
// subprocess. The caller MUST Close the reader to avoid leaking a 7z
// process.
func Stream(ctx context.Context, opts ListOpts, inner string) (io.ReadCloser, error) {
	bin, err := exec.LookPath(Binary)
	if err != nil {
		return nil, fmt.Errorf("7z binary not found in PATH: %w", err)
	}
	if opts.Src == "" || inner == "" {
		return nil, errors.New("sevenz.Stream: src and inner required")
	}

	args := []string{"e", "-so", "-bb0", "-y"}
	if opts.Password != "" {
		args = append(args, "-p"+opts.Password)
	} else {
		args = append(args, "-p")
	}
	args = append(args, "--", opts.Src, inner)

	klog.V(4).Infof("[sevenz] Stream: %s %s", bin, strings.Join(redactArgs(args), " "))

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = nil

	if startErr := cmd.Start(); startErr != nil {
		return nil, fmt.Errorf("start 7z: %w", startErr)
	}
	return &streamReader{rc: stdout, cmd: cmd}, nil
}

type streamReader struct {
	rc  io.ReadCloser
	cmd *exec.Cmd
}

func (s *streamReader) Read(p []byte) (int, error) { return s.rc.Read(p) }

func (s *streamReader) Close() error {
	closeErr := s.rc.Close()
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
	}
	return closeErr
}

// ----------------------------------------------------------------------
// Common subprocess runner with progress parsing
// ----------------------------------------------------------------------

// runWithProgress spawns 7z with the supplied args, parses progress
// lines off stdout (`-bsp1` puts percent on stdout), and invokes prog
// for each new percent. Returns Classify'd error on failure.
func runWithProgress(ctx context.Context, bin string, args []string, workdir string, prog ProgressFn) error {
	klog.V(2).Infof("[sevenz] run: %s %s", bin, strings.Join(redactArgs(args), " "))

	cmd := exec.CommandContext(ctx, bin, args...)
	if workdir != "" {
		cmd.Dir = workdir
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if startErr := cmd.Start(); startErr != nil {
		return fmt.Errorf("start 7z: %w", startErr)
	}

	var stderrBuf strings.Builder
	var wg sync.WaitGroup
	wg.Add(2)

	// stderr collector
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stderrBuf, stderr)
	}()

	// stdout progress parser. 7z emits progress lines like:
	//   "  3% 4 + foo/bar.txt"
	// and rewrites the line via \r when stdout is a TTY. To handle
	// both cases we split on \n OR \r.
	lastPercent := -1
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		sc.Split(scanCRLines)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			p, ok := parsePercent(line)
			if !ok {
				continue
			}
			if p != lastPercent {
				lastPercent = p
				if prog != nil {
					prog(p, 0)
				}
			}
		}
	}()

	waitErr := cmd.Wait()
	wg.Wait()

	if waitErr != nil {
		out := stderrBuf.String()
		klog.V(2).Infof("[sevenz] exit err: %v; stderr: %s", waitErr, common.RemoveBlank(out))
		return Classify(waitErr, out)
	}
	if prog != nil && lastPercent != 100 {
		prog(100, 0)
	}
	return nil
}

// scanCRLines is a bufio.SplitFunc that splits on either \r or \n so
// that 7z's progress reflow updates are surfaced as individual lines.
func scanCRLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i, b := range data {
		if b == '\n' || b == '\r' || b == '\b' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// parsePercent extracts the leading percentage from a 7z progress line.
// Returns (percent, true) for "  3% ..." and (0, false) for non-progress
// lines like "Scanning the drive:" / "Creating archive: foo.zip".
func parsePercent(line string) (int, bool) {
	// Find the first '%' and walk backwards collecting digits.
	idx := strings.Index(line, "%")
	if idx <= 0 {
		return 0, false
	}
	start := idx
	for start > 0 && line[start-1] >= '0' && line[start-1] <= '9' {
		start--
	}
	if start == idx {
		return 0, false
	}
	n, err := strconv.Atoi(line[start:idx])
	if err != nil {
		return 0, false
	}
	if n < 0 || n > 100 {
		return 0, false
	}
	return n, true
}

// removeIfExists deletes a path, swallowing not-exists errors. Defined
// here (rather than importing os in callers) so test substitution is
// trivial.
var removeIfExists = func(path string) error {
	return removeFn(path)
}

package tasks

import (
	"errors"
	"files/pkg/archive/sevenz"
	"files/pkg/common"
	"files/pkg/files"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/klog/v2"
)

// Compress is the Task phase function used by Archive compress requests.
// It expects t.param.Srcs / t.param.Dst / t.param.Archive to be set; the
// driver layer is responsible for populating them.
//
// Progress is mapped to the existing t.updateProgressRsync sink so the FE
// shares one progress UI for copy / move / compress.
func (t *Task) Compress() error {
	if err := t.validateArchiveCompress(); err != nil {
		return err
	}

	owner := t.param.Owner
	srcs := t.param.Srcs
	dst := t.param.Dst
	opt := t.param.Archive

	dstUri, err := dst.GetResourceUri()
	if err != nil {
		return fmt.Errorf("get dst uri: %w", err)
	}
	dstPath := dstUri + dst.Path
	dstDir := filepath.Dir(dstPath)

	klog.Infof("[Task] Id: %s, compress, user: %s, format: %s, dst: %s, srcs: %d",
		t.id, owner, opt.Format, dstPath, len(srcs))

	// Resolve every source to an absolute filesystem path and total
	// uncompressed size for the progress bar / disk-space precheck.
	absSrcs := make([]string, 0, len(srcs))
	var totalSize int64
	for _, s := range srcs {
		uri, e := s.GetResourceUri()
		if e != nil {
			return fmt.Errorf("get src uri: %w", e)
		}
		abs := uri + s.Path
		meta, e := files.GetFileInfo(abs)
		if e != nil {
			return fmt.Errorf("stat %s: %w", abs, e)
		}
		absSrcs = append(absSrcs, abs)
		totalSize += meta.Size
	}
	t.updateTotalSize(totalSize)

	// Disk space precheck: estimate output ~ 0.8 * uncompressed (we
	// can't know the real ratio without compressing, so 0.8 keeps us
	// on the safe side of refusing borderline cases).
	estimated := totalSize * 8 / 10
	if estimated < 1 {
		estimated = 1
	}
	if _, e := common.CheckDiskSpace(dstDir, estimated, dst.IsSystem()); e != nil {
		return e
	}

	// Conflict handling: if dst already exists, fall through to the
	// existing dup-name generator (matches paste behaviour). Multi-
	// volume outputs check the .001 sibling.
	if !t.pausedSnap().WasPaused {
		isVolume := opt.VolumeSizeMB > 0
		if newName, newPath, e := t.generateArchiveDstName(dstPath, dstUri, isVolume); e != nil {
			return e
		} else if newName != "" {
			dst.Path = newPath
			dstPath = dstUri + dst.Path
		}
	}

	// Make sure the parent dir exists. The HTTP precheck verifies the
	// path is writable but the directory itself may not yet exist for
	// freshly created sibling folders.
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dstDir, err)
	}

	// Build the 7z opts.
	cmpOpts := sevenz.CompressOpts{
		Dst:              dstPath,
		Sources:          absSrcs,
		Format:           opt.Format,
		Level:            opt.Level,
		Password:         opt.Password,
		HeaderEncrypt:    opt.HeaderEncrypt,
		VolumeSizeBytes:  opt.VolumeSizeMB * 1024 * 1024,
		PreserveSymlinks: opt.PreserveSymlinks,
	}

	// Best-effort cleanup of half-written archives if the run is
	// cancelled or errors. We do this inline (rather than in
	// task.Cancel) because Cancel's current branches are wired to the
	// rclone / sync backends; mixing posix cleanup there would broaden
	// the blast radius.
	defer func() {
		state := t.getState()
		if state == common.Canceled || state == common.Failed {
			cleanupArchiveOutputs(dstPath)
		}
	}()

	progFn := func(percent int, _ int64) {
		// 7z gives reliable percent only; transferred is derived from
		// totalSize when known.
		var transferred int64
		if totalSize > 0 {
			transferred = totalSize * int64(percent) / 100
		}
		t.updateProgressRsync(percent, transferred)
	}

	if err := sevenz.Compress(t.ctx, cmpOpts, progFn); err != nil {
		if cerr := t.ctx.Err(); cerr != nil {
			return cerr
		}
		return err
	}

	// Match the rsync path: chown the produced archive(s) to 1000:1000
	// so they show up under the owner's quota.
	for _, p := range archiveOutputPaths(dstPath) {
		if e := files.Chown(nil, p, 1000, 1000); e != nil {
			klog.Warningf("[Task] Id: %s, chown %s: %v", t.id, p, e)
		}
	}

	klog.Infof("[Task] Id: %s, compress done!", t.id)
	return nil
}

// Extract is the Task phase function used by Archive extract requests.
// It expects t.param.Src (archive file) / t.param.Dst (target dir) /
// t.param.Archive to be set.
func (t *Task) Extract() error {
	if err := t.validateArchiveExtract(); err != nil {
		return err
	}

	owner := t.param.Owner
	src := t.param.Src
	dst := t.param.Dst
	opt := t.param.Archive

	srcUri, err := src.GetResourceUri()
	if err != nil {
		return fmt.Errorf("get src uri: %w", err)
	}
	srcPath := srcUri + src.Path

	dstUri, err := dst.GetResourceUri()
	if err != nil {
		return fmt.Errorf("get dst uri: %w", err)
	}
	dstPath := dstUri + dst.Path

	klog.Infof("[Task] Id: %s, extract, user: %s, src: %s, dst: %s, format: %s",
		t.id, owner, srcPath, dstPath, opt.Format)

	// Estimate total uncompressed size by walking the archive metadata
	// once. This second pass costs little (just headers) and gives us
	// an honest progress denominator plus a real disk-space precheck.
	var totalSize int64
	if err := sevenz.Walk(t.ctx, sevenz.ListOpts{Src: srcPath, Password: opt.Password}, func(e sevenz.Entry) error {
		if !e.IsDir {
			totalSize += e.Size
		}
		return nil
	}); err != nil {
		// Walk errors are already Classify'd by sevenz.
		return err
	}
	t.updateTotalSize(totalSize)

	if _, e := common.CheckDiskSpace(dstPath, totalSize, dst.IsSystem()); e != nil {
		return e
	}

	// out/ exists -> out (1)/, out (2)/, ...
	if !t.pausedSnap().WasPaused {
		if newPath, e := t.generateExtractDstDir(dstPath, dstUri); e != nil {
			return e
		} else if newPath != "" {
			dst.Path = newPath
			dstPath = dstUri + dst.Path
		}
	}

	if err := os.MkdirAll(dstPath, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dstPath, err)
	}

	defer func() {
		state := t.getState()
		if state == common.Canceled || state == common.Failed {
			_ = os.RemoveAll(dstPath)
		}
	}()

	progFn := func(percent int, _ int64) {
		var transferred int64
		if totalSize > 0 {
			transferred = totalSize * int64(percent) / 100
		}
		t.updateProgressRsync(percent, transferred)
	}

	extOpts := sevenz.ExtractOpts{
		Src:              srcPath,
		Dst:              dstPath,
		Password:         opt.Password,
		PreserveSymlinks: opt.PreserveSymlinks,
		Overwrite:        opt.Conflict,
	}
	if err := sevenz.Extract(t.ctx, extOpts, progFn); err != nil {
		if cerr := t.ctx.Err(); cerr != nil {
			return cerr
		}
		return err
	}

	if err := files.ChownRecursive(dstPath, 1000, 1000); err != nil {
		klog.Warningf("[Task] Id: %s, chown %s: %v", t.id, dstPath, err)
	}

	klog.Infof("[Task] Id: %s, extract done!", t.id)
	return nil
}

// ----------------------------------------------------------------------
// helpers
// ----------------------------------------------------------------------

func (t *Task) validateArchiveCompress() error {
	if t.param == nil || t.param.Archive == nil || t.param.Dst == nil {
		return errors.New("invalid compress task param")
	}
	if len(t.param.Srcs) == 0 {
		return errors.New("compress task requires at least one source")
	}
	return nil
}

func (t *Task) validateArchiveExtract() error {
	if t.param == nil || t.param.Archive == nil || t.param.Src == nil || t.param.Dst == nil {
		return errors.New("invalid extract task param")
	}
	return nil
}

// generateArchiveDstName returns ("","",nil) when no collision, or
// ("foo (1).zip","/Home/foo (1).zip", nil) when a unique sibling name
// is found. For multi-volume archives the .001 first volume controls
// collision detection.
func (t *Task) generateArchiveDstName(dstPath, dstUri string, isVolume bool) (string, string, error) {
	probe := dstPath
	if isVolume {
		probe = dstPath + ".001"
	}
	if _, err := os.Stat(probe); errors.Is(err, os.ErrNotExist) {
		return "", "", nil
	} else if err != nil {
		return "", "", err
	}
	base := filepath.Base(dstPath)
	_, ext := common.SplitNameExt(base)
	name := strings.TrimSuffix(base, ext)
	dir := filepath.Dir(dstPath)
	siblings, err := files.CollectDupNames(dir, name, ext, false)
	if err != nil {
		return "", "", err
	}
	newName := files.GenerateDupName(siblings, base, true)
	if newName == base {
		return "", "", nil
	}
	newPath := strings.TrimPrefix(dir+"/"+newName, dstUri)
	return newName, newPath, nil
}

// generateExtractDstDir returns "" when dstPath is free, else a renamed
// sibling path (out -> out (1) -> out (2)).
func (t *Task) generateExtractDstDir(dstPath, dstUri string) (string, error) {
	if _, err := os.Stat(dstPath); errors.Is(err, os.ErrNotExist) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	name := filepath.Base(dstPath)
	dir := filepath.Dir(dstPath)
	siblings, err := files.CollectDupNames(dir, name, "", true)
	if err != nil {
		return "", err
	}
	newName := files.GenerateDupName(siblings, name, false)
	if newName == name {
		return "", nil
	}
	return strings.TrimPrefix(dir+"/"+newName, dstUri), nil
}

// archiveOutputPaths returns all on-disk files that compose the
// archive at dstPath: the bare file plus any .NNN volume parts.
func archiveOutputPaths(dstPath string) []string {
	out := []string{dstPath}
	// Walk siblings dstPath.001, .002, ... until a gap is found.
	for i := 1; i < 1000; i++ {
		p := fmt.Sprintf("%s.%03d", dstPath, i)
		if _, err := os.Stat(p); err != nil {
			break
		}
		out = append(out, p)
	}
	return out
}

func cleanupArchiveOutputs(dstPath string) {
	for _, p := range archiveOutputPaths(dstPath) {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			klog.Warningf("[Task] cleanup archive %s: %v", p, err)
		}
	}
}

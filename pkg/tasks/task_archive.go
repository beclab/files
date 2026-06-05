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
func (t *Task) Compress() (retErr error) {
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

	if dst.FileType == common.External {
		if err = t.checkExternalDstMountAlive(); err != nil {
			return fmt.Errorf("check external dst mount alive error: %v", err)
		}
	}

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

	// Conflict handling for an existing archive at dst (overwrite backups restored by defer below on cancel/failure).
	var overwriteBackups [][2]string
	if !t.pausedSnap().WasPaused {
		isVolume := opt.VolumeSizeMB > 0
		switch opt.Conflict {
		case common.ArchiveConflictOverwrite:
			for _, p := range archiveOutputPaths(dstPath) {
				bak := p + ".bak." + t.id
				if e := os.Rename(p, bak); e != nil {
					if os.IsNotExist(e) {
						continue
					}
					for _, b := range overwriteBackups {
						_ = os.Rename(b[1], b[0])
					}
					return fmt.Errorf("backup %s: %w", p, e)
				}
				overwriteBackups = append(overwriteBackups, [2]string{p, bak})
			}
		case common.ArchiveConflictSkip:
			existing := dstPath
			if isVolume {
				existing = dstPath + ".001"
			}
			if _, e := os.Stat(existing); e == nil {
				klog.Infof("[Task] Id: %s, compress, skip existing archive: %s", t.id, existing)
				return nil
			} else if !errors.Is(e, os.ErrNotExist) {
				return e
			}
		default: // rename or unset
			if newName, newPath, e := t.generateArchiveDstName(dstPath, dstUri, isVolume); e != nil {
				return e
			} else if newName != "" {
				dst.Path = newPath
				dstPath = dstUri + dst.Path
			}
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
		// state lags behind retErr (Execute sets Canceled/Failed only after we return), so trust retErr.
		if retErr != nil {
			cleanupArchiveOutputs(dstPath)
			for _, b := range overwriteBackups {
				_ = os.Rename(b[1], b[0])
			}
		} else {
			for _, b := range overwriteBackups {
				_ = os.Remove(b[1])
			}
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
func (t *Task) Extract() (retErr error) {
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

	if dst.FileType == common.External {
		if err = t.checkExternalDstMountAlive(); err != nil {
			return fmt.Errorf("check external dst mount alive error: %v", err)
		}
	}

	klog.Infof("[Task] Id: %s, extract, user: %s, src: %s, dst: %s, format: %s",
		t.id, owner, srcPath, dstPath, opt.Format)

	// Estimate total uncompressed size by walking the archive metadata
	// once. This second pass costs little (just headers) and gives us
	// an honest progress denominator plus a real disk-space precheck.
	// Compound tar enumerate-by-walk is as costly as extract itself; defer top-name decisions to post-extract staging.
	isCompound := sevenz.IsCompoundTar(srcPath)
	var totalSize int64
	topIsDir := map[string]bool{}
	if !isCompound {
		if err := sevenz.Walk(t.ctx, sevenz.ListOpts{Src: srcPath, Password: opt.Password}, func(e sevenz.Entry) error {
			if !e.IsDir {
				totalSize += e.Size
			}
			n := strings.TrimPrefix(filepath.ToSlash(e.Path), "/")
			if n == "" || n == "." {
				return nil
			}
			parts := strings.SplitN(n, "/", 2)
			seg := parts[0]
			if len(parts) > 1 {
				topIsDir[seg] = true
			} else if _, ok := topIsDir[seg]; !ok {
				topIsDir[seg] = e.IsDir
			}
			return nil
		}); err != nil {
			// Walk errors are already Classify'd by sevenz.
			return err
		}
	}
	// xz / bzip2 / tar.xz / tar.bz2 stream formats don't expose unpacked size; fall back to packed src size.
	if totalSize == 0 {
		if fi, err := os.Stat(srcPath); err == nil {
			totalSize = fi.Size()
		}
	}
	t.updateTotalSize(totalSize)

	spaceEstimate := totalSize
	if isCompound {
		spaceEstimate = totalSize * 3
	}
	if _, e := common.CheckDiskSpace(dstPath, spaceEstimate, dst.IsSystem()); e != nil {
		return e
	}

	var renamePlan map[string]string
	if !isCompound && (opt.Conflict == "" || opt.Conflict == common.ArchiveConflictRename) {
		plan := map[string]string{}
		for top, isDir := range topIsDir {
			if _, e := os.Stat(filepath.Join(dstPath, top)); e != nil {
				continue
			}
			var newTop string
			if isDir {
				siblings, _ := files.CollectDupNames(dstPath, top, "", true)
				n := files.GenerateDupName(siblings, top, false)
				if n != "" && n != top {
					newTop = n
				}
			} else {
				_, ext := common.SplitNameExt(top)
				base := strings.TrimSuffix(top, ext)
				siblings, _ := files.CollectDupNames(dstPath, base, ext, false)
				n := files.GenerateDupName(siblings, base, true)
				if n != "" && n != base {
					newTop = n + ext
				}
			}
			if newTop != "" {
				plan[top] = newTop
			}
		}
		if len(plan) > 0 {
			renamePlan = plan
		}
	}

	extractRoot := dstPath
	var stagingDir string
	if isCompound || renamePlan != nil {
		stagingDir = filepath.Join(dstPath, fmt.Sprintf(".archive_extract.%s", t.id))
		if e := os.MkdirAll(stagingDir, 0o755); e != nil {
			return fmt.Errorf("mkdir staging %s: %w", stagingDir, e)
		}
		extractRoot = stagingDir
	} else {
		// out/ exists -> out (1)/, out (2)/, ...
		if !t.pausedSnap().WasPaused {
			if newPath, e := t.generateExtractDstDir(dstPath, dstUri); e != nil {
				return e
			} else if newPath != "" {
				dst.Path = newPath
				dstPath = dstUri + dst.Path
				extractRoot = dstPath
			}
		}
		if err := os.MkdirAll(dstPath, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", dstPath, err)
		}
	}

	defer func() {
		// state lags behind retErr (Execute sets Canceled/Failed only after we return), so trust retErr.
		if retErr != nil {
			if stagingDir != "" {
				_ = os.RemoveAll(stagingDir)
			} else {
				_ = os.RemoveAll(dstPath)
			}
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
		Dst:              extractRoot,
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

	var chownRoots []string
	if stagingDir != "" {
		// Compound tar overwrite / skip merge per file so dst files outside the archive's tree stay untouched.
		if isCompound && (opt.Conflict == common.ArchiveConflictOverwrite || opt.Conflict == common.ArchiveConflictSkip) {
			walkErr := filepath.Walk(stagingDir, func(srcPath string, info os.FileInfo, werr error) error {
				if werr != nil {
					return werr
				}
				if srcPath == stagingDir {
					return nil
				}
				rel, _ := filepath.Rel(stagingDir, srcPath)
				dstFile := filepath.Join(dstPath, rel)
				if info.IsDir() {
					if err := os.MkdirAll(dstFile, info.Mode()); err != nil {
						return err
					}
					return nil
				}
				if _, st := os.Stat(dstFile); st == nil {
					if opt.Conflict == common.ArchiveConflictSkip {
						_ = os.Remove(srcPath)
						return nil
					}
					if err := os.RemoveAll(dstFile); err != nil {
						return err
					}
				}
				if err := os.MkdirAll(filepath.Dir(dstFile), 0o755); err != nil {
					return err
				}
				if err := os.Rename(srcPath, dstFile); err != nil {
					return err
				}
				chownRoots = append(chownRoots, dstFile)
				return nil
			})
			if walkErr != nil {
				return fmt.Errorf("merge: %w", walkErr)
			}
			_ = os.RemoveAll(stagingDir)
		} else {
			entries, err := os.ReadDir(stagingDir)
			if err != nil {
				return err
			}
			// Compound tar: now that the real top-level entries exist on disk, decide rename per entry.
			if isCompound && (opt.Conflict == "" || opt.Conflict == common.ArchiveConflictRename) {
				plan := map[string]string{}
				for _, e := range entries {
					top := e.Name()
					if _, st := os.Stat(filepath.Join(dstPath, top)); st != nil {
						continue
					}
					var newTop string
					if e.IsDir() {
						siblings, _ := files.CollectDupNames(dstPath, top, "", true)
						n := files.GenerateDupName(siblings, top, false)
						if n != "" && n != top {
							newTop = n
						}
					} else {
						_, ext := common.SplitNameExt(top)
						base := strings.TrimSuffix(top, ext)
						siblings, _ := files.CollectDupNames(dstPath, base, ext, false)
						n := files.GenerateDupName(siblings, base, true)
						if n != "" && n != base {
							newTop = n + ext
						}
					}
					if newTop != "" {
						plan[top] = newTop
					}
				}
				if len(plan) > 0 {
					renamePlan = plan
				}
			}
			for _, e := range entries {
				name := e.Name()
				target := name
				if newName, ok := renamePlan[name]; ok {
					target = newName
				}
				to := filepath.Join(dstPath, target)
				if err := os.Rename(filepath.Join(stagingDir, name), to); err != nil {
					return fmt.Errorf("merge %s -> %s: %w", name, target, err)
				}
				chownRoots = append(chownRoots, to)
			}
			_ = os.Remove(stagingDir)
		}
	} else {
		chownRoots = []string{dstPath}
	}

	for _, p := range chownRoots {
		if err := files.ChownRecursive(p, 1000, 1000); err != nil {
			klog.Warningf("[Task] Id: %s, chown %s: %v", t.id, p, err)
		}
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
	newPrefix := files.GenerateDupName(siblings, name, true)
	if newPrefix == name {
		return "", "", nil
	}
	newName := newPrefix + ext
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
	// Sweep multi-volume 7z leftovers: dstPath.tmp / dstPath.NNN.tmp / dstPath.NNN.<hash>.tmp.
	if matches, _ := filepath.Glob(dstPath + "*.tmp"); matches != nil {
		for _, p := range matches {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				klog.Warningf("[Task] cleanup archive tmp %s: %v", p, err)
			}
		}
	}
}

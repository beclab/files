package common

import (
	"bytes"
	"context"
	"files/pkg/errors"
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/pool"
	"files/pkg/preview"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"
	"k8s.io/klog/v2"
)

func CheckParent(src, dst string) error {
	rel, err := filepath.Rel(src, dst)
	if err != nil {
		return err
	}

	rel = filepath.ToSlash(rel)
	if !strings.HasPrefix(rel, "../") && rel != ".." && rel != "." {
		return errors.ErrSourceIsParent
	}

	return nil
}

func AddVersionSuffix(source string, fs afero.Fs, isDir bool) string {
	counter := 1
	dir, name := path.Split(source)
	ext := ""
	base := name
	if !isDir {
		ext = filepath.Ext(name)
		base = strings.TrimSuffix(name, ext)
	}

	for {
		if fs == nil {
			if _, err := os.Stat(source); err != nil {
				break
			}
		} else {
			if _, err := fs.Stat(source); err != nil {
				break
			}
		}
		renamed := fmt.Sprintf("%s(%d)%s", base, counter, ext)
		source = path.Join(dir, renamed)
		counter++
	}

	return source
}

func pathExistsInLsOutput(path string, isDir bool) (bool, error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	cmd := exec.Command("ls", "-l", dir)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		klog.Errorf("ls -l %s failed: %v, stderr: %s", path, err, stderr.String())
	}

	lsOutput := out.String()

	lines := strings.Split(lsOutput, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		fileType := line[:1]

		parts := strings.Fields(line)
		if len(parts) < 9 {
			continue
		}
		fileName := parts[len(parts)-1]

		if fileName == base {
			if fileType == "d" && isDir {
				return true, nil
			} else if fileType == "-" && !isDir {
				return true, nil
			}
		}
	}
	return false, nil
}

func Rmrf(path string) error {
	for {
		cmd := exec.Command("rm", "-rf", path)

		cmdStr := fmt.Sprintf("%s %s", cmd.Path, strings.Join(cmd.Args[1:], " "))
		klog.Infoln("Executing command:", cmdStr)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			klog.Infoln("Error executing rm -rf:", err)
		}
		klog.Infoln("Command stdout:", stdout.String())
		klog.Infoln("Command stderr:", stderr.String())

		exists, err := pathExistsInLsOutput(path, true)
		if !exists && err == nil {
			klog.Infoln("Path successfully removed:", path)
			return nil
		} else if err != nil {
			klog.Infoln("Error checking path existence:", err)
		} else {
			klog.Infoln("Path still exists, retrying...")
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func safeRemoveAll(fs afero.Fs, source string) error {
	// TODO: not used for the time being
	realPath := filepath.Join(RootPrefix, source)
	for {
		err := fs.RemoveAll(source)
		if err != nil {
			klog.Errorf("Error attempting to delete folder %s: %v", realPath, err)
		} else {
			exists, err := pathExistsInLsOutput(filepath.Join(RootPrefix, source), true)
			if !exists && err == nil {
				klog.Infof("Successfully deleted folder %s", realPath)
				return nil
			} else if err != nil {
				klog.Errorf("Error checking folder existence after deletion attempt for %s: %v", realPath, err)
			} else {
				klog.Warningf("Folder %s still exists after deletion attempt, retrying...", realPath)
			}
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func MoveDir(ctx context.Context, fs afero.Fs, source, dest, srcExternalType, dstExternalType string, fileCache fileutils.FileCache, delete bool) error {
	var err error
	if delete {
		// first recursively delete all thumbs
		err = filepath.Walk(RootPrefix+source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				file, err := files.NewFileInfo(files.FileOptions{
					Fs:         files.DefaultFs,
					Path:       path,
					Modify:     true,
					Expand:     false,
					ReadHeader: false,
				})
				if err != nil {
					return err
				}

				// delete thumbnails
				err = preview.DelThumbs(ctx, fileCache, file)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			klog.Infoln("Error walking the directory:", err)
		} else {
			klog.Infoln("Directory traversal completed.")
		}
	}

	// no matter what situation, try to rename all first
	if fs.Rename(source, dest) == nil {
		return nil
	}

	// if rename all failed, recursively do things below, without delthumbs any more

	// Get properties of source.
	srcinfo, err := fs.Stat(source)
	if err != nil {
		return err
	}

	// Create the destination directory.
	if err = fileutils.MkdirAllWithChown(fs, dest, srcinfo.Mode()); err != nil {
		klog.Errorln(err)
		return err
	}

	dir, _ := fs.Open(source)
	obs, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	var errs []error

	for _, obj := range obs {
		fsource := filepath.Join(source, obj.Name())
		fdest := filepath.Join(dest, obj.Name())

		if obj.IsDir() {
			// Create sub-directories, recursively.
			err = MoveDir(ctx, fs, fsource, fdest, srcExternalType, dstExternalType, fileCache, false)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			// Perform the file copy.
			err = MoveFile(ctx, fs, fsource, fdest, fileCache, false)
			if err != nil {
				errs = append(errs, err)
			}
		}
	}

	var errString string
	for _, err := range errs {
		errString += err.Error() + "\n"
	}

	if errString != "" {
		klog.Infof("Rollbacking: Dest ExternalType is %s", dstExternalType)
		if dstExternalType == "smb" {
			err = Rmrf(RootPrefix + dest)
		} else {
			err = fs.RemoveAll(dest)
		}
		if err != nil {
			errString += err.Error() + "\n"
		}
		return fmt.Errorf(errString)
	}

	// finally delete all for folder is OK
	if delete {
		klog.Infof("Moving is going to Delete folder %s with ExternalType %s", RootPrefix+source, srcExternalType)
		if srcExternalType == "smb" {
			err = Rmrf(RootPrefix + source)
		} else {
			err = fs.RemoveAll(source)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func MoveFile(ctx context.Context, fs afero.Fs, src, dst string, fileCache fileutils.FileCache, delete bool) error {
	src = path.Clean("/" + src)
	dst = path.Clean("/" + dst)

	if delete {
		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       src,
			Modify:     true,
			Expand:     false,
			ReadHeader: false,
		})
		if err != nil {
			return err
		}

		// delete thumbnails
		err = preview.DelThumbs(ctx, fileCache, file)
		if err != nil {
			return err
		}
	}

	return fileutils.MoveFile(fs, src, dst)
}

func Move(ctx context.Context, fs afero.Fs, task *pool.Task, src, dst, srcExternalType, dstExternalType string, fileCache fileutils.FileCache) error {
	if src = path.Clean("/" + src); src == "" {
		return os.ErrNotExist
	}

	if dst = path.Clean("/" + dst); dst == "" {
		return os.ErrNotExist
	}

	if src == "/" || dst == "/" {
		// Prohibit copying from or to the virtual root directory.
		return os.ErrInvalid
	}

	if dst == src {
		return os.ErrInvalid
	}

	info, err := fs.Stat(src)
	if err != nil {
		return err
	}

	klog.Infof("copy %v from %s to %s", info, src, dst)

	if task == nil {
		if info.IsDir() {
			return MoveDir(ctx, fs, src, dst, srcExternalType, dstExternalType, fileCache, true)
		}

		return MoveFile(ctx, fs, src, dst, fileCache, true)
	}

	go func() {
		var err error
		err = ExecuteMoveWithRsync(task, srcExternalType, dstExternalType, fileCache)
		if err != nil {
			klog.Errorf("Failed to initialize rsync: %v\n", err)
			return
		}
	}()
	return nil
}

func PatchAction(task *pool.Task, ctx context.Context, action, src, dst, srcExternalType, dstExternalType string, fileCache fileutils.FileCache) error {
	switch action {
	case "copy":
		return fileutils.Copy(files.DefaultFs, task, src, dst)
	case "rename", "move":
		return Move(ctx, files.DefaultFs, task, src, dst, srcExternalType, dstExternalType, fileCache)
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}
}

func ExecuteMoveWithRsync(task *pool.Task, srcExternalType, dstExternalType string, fileCache fileutils.FileCache) error {
	srcinfo, err := files.DefaultFs.Stat(task.Source)
	if err != nil {
		return err
	}

	if srcinfo.IsDir() {
		// first recursively delete all thumbs
		err = filepath.Walk(RootPrefix+task.Source, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				file, err := files.NewFileInfo(files.FileOptions{
					Fs:         files.DefaultFs,
					Path:       path,
					Modify:     true,
					Expand:     false,
					ReadHeader: false,
				})
				if err != nil {
					return err
				}

				// delete thumbnails
				err = preview.DelThumbs(task.Ctx, fileCache, file)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			klog.Infoln("Error walking the directory:", err)
		} else {
			klog.Infoln("Directory traversal completed.")
		}
	} else {
		file, err := files.NewFileInfo(files.FileOptions{
			Fs:         files.DefaultFs,
			Path:       task.Source,
			Modify:     true,
			Expand:     false,
			ReadHeader: false,
		})
		if err != nil {
			return err
		}

		// delete thumbnails
		err = preview.DelThumbs(task.Ctx, fileCache, file)
		if err != nil {
			return err
		}
	}

	// no matter what situation, try to rename all first
	if files.DefaultFs.Rename(task.Source, task.Dest) == nil {
		task.Logging(fmt.Sprintf("rename from %s to %s successfully", task.Source, task.Dest))
		pool.CompleteTask(task.ID)
		return nil
	}
	// if rename all failed, recursively do things below, without delthumbs any more

	// Get properties of source.
	go func() {
		err = fileutils.ExecuteRsync(task, "", "", 0, 99)
		if err != nil {
			klog.Errorf("Failed to initialize rsync: %v\n", err)
			pool.FailTask(task.ID)
			return
		}
		if srcExternalType == "smb" {
			err = Rmrf(RootPrefix + task.Source)
		} else {
			err = files.DefaultFs.RemoveAll(task.Source)
		}
		if err != nil {
			klog.Errorf("Failed to remove %v: %v", task.Source, err)
			return
		}
		pool.CompleteTask(task.ID)
	}()
	return nil
}

package fileutils

import (
	"errors"
	"k8s.io/klog/v2"

	"github.com/spf13/afero"
)

// CopyDir copies a directory from source to dest and all
// of its sub-directories. It doesn't stop if it finds an error
// during the copy. Returns an error if any.
func CopyDir(fs afero.Fs, source, dest string) error {
	// Get properties of source.
	srcinfo, err := fs.Stat(source)
	if err != nil {
		return err
	}

	// Create the destination directory.
	if err = MkdirAllWithChown(fs, dest, srcinfo.Mode()); err != nil {
		klog.Errorln(err)
		return err
	}
	//if err = fs.MkdirAll(dest, srcinfo.Mode()); err != nil {
	//	return err
	//}
	//if err = Chown(fs, dest, 1000, 1000); err != nil {
	//	klog.Errorf("can't chown directory %s to user %d: %s", dest, 1000, err)
	//	return err
	//}

	dir, _ := fs.Open(source)
	obs, err := dir.Readdir(-1)
	if err != nil {
		return err
	}

	var errs []error

	for _, obj := range obs {
		fsource := source + "/" + obj.Name()
		fdest := dest + "/" + obj.Name()

		if obj.IsDir() {
			// Create sub-directories, recursively.
			err = CopyDir(fs, fsource, fdest)
			if err != nil {
				errs = append(errs, err)
			}
		} else {
			// Perform the file copy.
			err = CopyFile(fs, fsource, fdest)
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
		return errors.New(errString)
	}

	return nil
}

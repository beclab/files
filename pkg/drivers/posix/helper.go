package posix

import (
	"errors"
	"files/pkg/files"
	"os"

	"github.com/spf13/afero"
)

func getRawFile(file *files.FileInfo) (afero.File, error) {
	return file.Fs.Open(file.Path)
}

func extractErrMsg(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}

	var le *os.LinkError
	if errors.As(err, &le) {
		return le.Err.Error()
	}

	var se *os.SyscallError
	if errors.As(err, &se) {
		return se.Err.Error()
	}

	return err.Error()

	// for {
	// 	next := errors.Unwrap(err)
	// 	if next == nil {
	// 		return err.Error()
	// 	}
	// 	err = next
	// }
}

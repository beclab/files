package posix

import (
	"files/pkg/files"

	"github.com/spf13/afero"
)

func getRawFile(file *files.FileInfo) (afero.File, error) {
	return file.Fs.Open(file.Path)
}

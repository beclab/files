package posix

import (
	"files/pkg/files"
	"io"
)

func getRawFile(file *files.FileInfo) (io.ReadCloser, error) {
	return file.Fs.Open(file.Path)
}

package preview

import (
	"files/pkg/files"
	"files/pkg/fileutils"
	"files/pkg/settings"
	"net/http"
)

func HandlerTextPreview(w http.ResponseWriter,
	r *http.Request,
	fileCache fileutils.FileCache,
	file *files.FileInfo,
	server *settings.Server) ([]byte, error) {
	return RawFileHandler(file)
}

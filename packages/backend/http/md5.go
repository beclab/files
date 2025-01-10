package http

import (
	"errors"
	"github.com/filebrowser/filebrowser/v2/common"
	"github.com/filebrowser/filebrowser/v2/files"
	"net/http"
)

func md5FileHandler(w http.ResponseWriter, r *http.Request, file *files.FileInfo) (int, error) {
	fd, err := file.Fs.Open(file.Path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	responseData := make(map[string]interface{})
	responseData["md5"] = common.Md5File(fd)
	return renderJSON(w, r, responseData)
}

var md5Handler = withUser(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
	if !d.user.Perm.Download {
		return http.StatusAccepted, nil
	}

	file, err := files.NewFileInfo(files.FileOptions{
		Fs:         d.user.Fs,
		Path:       r.URL.Path,
		Modify:     d.user.Perm.Modify,
		Expand:     false,
		ReadHeader: d.server.TypeDetectionByHeader,
		Checker:    d,
	})
	if err != nil {
		return errToStatus(err), err
	}

	if file.IsDir {
		err = errors.New("only support md5 for file")
		return http.StatusForbidden, err
	}

	return md5FileHandler(w, r, file)
})

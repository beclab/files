package http

import (
	"encoding/json"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"net/http"
)

func pasteHandler() http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var pasteParam, err = models.NewPasteParam(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var user = pasteParam.Owner
		// Destination: /{fileType}/{extend}/{path}/
		dstFileParam, err := models.CreateFileParam(user, pasteParam.Destination)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		var dstFileType = dstFileParam.FileType
		var handler = drivers.Adaptor.NewFileHandler(dstFileType, &base.HandlerParam{})

		if err := handler.Paste(pasteParam); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
				"message": err.Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(nil)
		return

	})

	return handler
}

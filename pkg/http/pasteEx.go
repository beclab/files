package http

import (
	"encoding/json"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
	"files/pkg/utils"
	"net/http"
)

var wrapperPasteArgs = func(prefix string) http.Handler {
	return pasteHandle(prefix)
}

func pasteHandle(prefix string) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var pasteParam, err = models.NewPasteParam(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		handler := drivers.Adaptor.NewFileHandler(pasteParam.Src.FileType, &base.HandlerParam{})

		task, err := handler.Paste(pasteParam)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code":    1,
				"message": err.Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		var data = map[string]string{"task_id": task.Id()}
		w.Write([]byte(utils.ToJson(data)))
		return

	})

	return handler
}

package http

import (
	"encoding/json"
	"files/pkg/drivers"
	"files/pkg/drivers/base"
	"files/pkg/models"
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

		if err := drivers.Adaptor.Paste(pasteParam, &base.HandlerParam{
			Ctx:   r.Context(),
			Owner: pasteParam.Owner,
		}); err != nil {
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

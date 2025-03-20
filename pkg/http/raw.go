package http

import (
	"files/pkg/common"
	"files/pkg/drives"
	"net/http"
)

func rawHandler(w http.ResponseWriter, r *http.Request, d *common.Data) (int, error) {
	//start := time.Now()
	//klog.Infoln("Function rawHandler starts at", start)
	//defer func() {
	//	elapsed := time.Since(start)
	//	klog.Infof("Function rawHandler execution time: %v\n", elapsed)
	//}()

	srcType := r.URL.Query().Get("src")
	handler, err := drives.GetResourceService(srcType)
	if err != nil {
		return http.StatusBadRequest, err
	}

	return handler.RawHandler(w, r, d)
}

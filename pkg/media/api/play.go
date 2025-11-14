package api

import (
	"fmt"
	"net/http"
	//	"files/pkg/media/service"
)

/*
	func mainlist(w http.ResponseWriter, r *http.Request) {
		service.Dhc.GetVariantHlsVideoPlaylist(w, r)
	}
*/
func play(w http.ResponseWriter, r *http.Request) {
	fmt.Println("^^^^^^^^^^^^^^^^^^^^^^^^", r.URL.Path)
	// service.Dhc.GetHlsVideoSegment(w, r)
}

package api

import (
	"net/http"

	"files/pkg/common"
	"files/pkg/media/service"

	"github.com/gorilla/mux"

	"k8s.io/klog/v2"
)

func GetMasterHlsVideoPlaylist(w http.ResponseWriter, r *http.Request) {
	// service.GetDynamicHlsController().GetMasterHlsVideoPlaylist(w, r)
}

func GetVariantHlsVideoPlaylist(w http.ResponseWriter, r *http.Request) {
	// service.GetDynamicHlsController().GetVariantHlsVideoPlaylist(w, r)
}

func GetHlsVideoSegment(w http.ResponseWriter, r *http.Request) {
	// service.GetDynamicHlsController().GetHlsVideoSegment(w, r)
}

func GetNamedConfiguration(w http.ResponseWriter, r *http.Request) {
	// service.GetConfigurationController().GetNamedConfiguration(w, r)
}

func UpdateNamedConfiguration(w http.ResponseWriter, r *http.Request) {
	// service.GetConfigurationController().UpdateNamedConfiguration(w, r)
}

// setupCORS sets per-response CORS headers for the media subsystem.
// Origin is echoed only when common.AllowedOrigin authorizes it
// (same effective host, or a host listed in $CORS_ALLOWED_ORIGINS),
// matching the policy applied to the main HTTP server.
func setupCORS(w http.ResponseWriter, r *http.Request) {
	if allowed := common.AllowedOrigin(r.Header.Get("Origin"), r.Header.Get("X-Forwarded-Host"), r.Host); allowed != "" {
		w.Header().Set("Access-Control-Allow-Origin", allowed)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Add("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Headers", "access-control-allow-headers,access-control-allow-methods,access-control-allow-origin,content-type,x-auth,x-unauth-error,x-authorization")
	w.Header().Set("Access-Control-Allow-Methods", "PUT, GET, DELETE, POST, OPTIONS")
}

func StartHttpServer() {
	r := mux.NewRouter()

	// Handle preflight OPTIONS requests for all routes
	r.Methods(http.MethodOptions).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		klog.Infoln("OPTIONS")
		setupCORS(w, r)
		w.Header().Set("Access-Control-Max-Age", "1728000")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusNoContent)
	})

	r.HandleFunc("/videos/{node}/", service.GetCustomPlayController().Play).Methods("GET")

	r.HandleFunc("/videos/master.m3u8", GetMasterHlsVideoPlaylist).Methods("GET")
	r.HandleFunc("/videos/{node}/main.m3u8", GetVariantHlsVideoPlaylist).Methods("GET")
	r.HandleFunc("/videos/{node}/hls1/{playlistId}/{segmentId}.{container}", GetHlsVideoSegment).Methods("GET")

	r.HandleFunc("/System/Configuration/{key}", GetNamedConfiguration).Methods("GET")
	r.HandleFunc("/System/Configuration/{key}", UpdateNamedConfiguration).Methods("POST")

	// Apply CORS middleware to all routes
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			klog.Infof("Request: %s %s\n", r.Method, r.URL.Path)
			setupCORS(w, r)
			next.ServeHTTP(w, r)
		})
	})

	// Handle undefined routes (catch-all for 404)
	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "404 Not Found", http.StatusNotFound)
	})

	klog.Infoln("start http server")
	http.ListenAndServe(":9090", r)
}

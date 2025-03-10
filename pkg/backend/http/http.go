package http

import (
	"files/pkg/backend/rpc"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"

	"github.com/gorilla/mux"

	"files/pkg/backend/settings"
)

func NewHandler(
	imgSvc ImgService,
	fileCache FileCache,
	server *settings.Server,
) (http.Handler, error) {
	server.Clean()

	r := mux.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", `default-src 'self'; style-src 'unsafe-inline';`)
			next.ServeHTTP(w, r)
		})
	})

	// NOTE: This fixes the issue where it would redirect if people did not put a
	// trailing slash in the end. I hate this decision since this allows some awful
	// URLs https://www.gorillatoolkit.org/pkg/mux#Router.SkipClean
	r = r.SkipClean(true)

	monkey := func(fn handleFunc, prefix string) http.Handler {
		return handle(fn, prefix, server)
	}

	r.HandleFunc("/health", healthHandler)

	api := r.PathPrefix("/api").Subrouter()

	api.PathPrefix("/resources").Handler(monkey(resourceGetHandler, "/api/resources")).Methods("GET")
	api.PathPrefix("/resources").Handler(monkey(resourceDeleteHandler(fileCache), "/api/resources")).Methods("DELETE")
	api.PathPrefix("/resources").Handler(monkey(resourcePostHandler(fileCache), "/api/resources")).Methods("POST")
	api.PathPrefix("/resources").Handler(monkey(resourcePutHandler, "/api/resources")).Methods("PUT")
	api.PathPrefix("/resources").Handler(monkey(resourcePatchHandler(fileCache), "/api/resources")).Methods("PATCH")
	api.PathPrefix("/mount").Handler(monkey(resourceMountHandler, "/api/mount")).Methods("POST")
	api.PathPrefix("/unmount").Handler(monkey(resourceUnmountHandler, "/api/unmount")).Methods("DELETE")
	// Because /api/resources/AppData is proxied under current arch, new api must be of a different prefix,
	// and try to access /api/resources/AppData in the handle func.
	api.PathPrefix("/paste").Handler(monkey(resourcePasteHandler(fileCache), "/api/paste")).Methods("PATCH")

	api.PathPrefix("/share/shareable").Handler(monkey(shareableGetHandler, "/api/share/shareable")).Methods("GET")
	api.PathPrefix("/share/shareable").Handler(monkey(shareablePutHandler, "/api/share/shareable")).Methods("PUT")
	api.PathPrefix("/share/share_link").Handler(monkey(shareLinkGetHandler, "/api/share/share_link")).Methods("GET")
	api.PathPrefix("/share/share_link").Handler(monkey(shareLinkPostHandler, "/api/share/share_link")).Methods("POST")
	api.PathPrefix("/share/share_link").Handler(monkey(shareLinkDeleteHandler, "/api/share/share_link")).Methods("DELETE")

	api.PathPrefix("/raw").Handler(monkey(rawHandler, "/api/raw")).Methods("GET")
	api.PathPrefix("/md5").Handler(monkey(md5Handler, "/api/md5")).Methods("GET")
	api.PathPrefix("/smb_history").Handler(monkey(smbHistoryGetHandler, "/api/smb_history")).Methods("GET")
	api.PathPrefix("/smb_history").Handler(monkey(smbHistoryPutHandler, "/api/smb_history")).Methods("PUT")
	api.PathPrefix("/smb_history").Handler(monkey(smbHistoryDeleteHandler, "/api/smb_history")).Methods("DELETE")
	api.PathPrefix("/smb_history").Handler(monkey(smbHistoryDeleteHandler, "/api/smb_history")).Methods("PATCH")

	api.PathPrefix("/preview/{size}/{path:.*}").
		Handler(monkey(previewHandler(imgSvc, fileCache, server.EnableThumbnails, server.ResizePreview), "/api/preview")).Methods("GET")

	files := r.PathPrefix("/files").Subrouter()
	files.HandleFunc("/healthcheck", ginHandlerAdapter(rpc.RpcEngine))

	return stripPrefix(server.BaseURL, r), nil
}

func ginHandlerAdapter(ginEngine *gin.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("You see me~")
		fmt.Println("request: ", r)
		fmt.Println("request header: ", r.Header)
		fmt.Println("request body: ", r.Body)
		ginEngine.ServeHTTP(w, r)
	}
}

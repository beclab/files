package http

import (
	"files/pkg/drivers"
	"files/pkg/fileutils"
	"files/pkg/preview"
	"files/pkg/rpc"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/gorilla/mux"

	"files/pkg/settings"
)

func NewHandler(
	imgSvc preview.ImgService,
	fileCache fileutils.FileCache,
	driverHandler *drivers.DriverHandler,
	server *settings.Server,
) (http.Handler, error) {
	server.Clean()

	r := mux.NewRouter()
	r.Use(timingMiddleware)
	r.Use(cookieMiddleware)
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

	common := func(fn commonFunc) http.Handler {
		return commonHandle(fn)
	}

	monkey := func(fn handleFunc, prefix string) http.Handler {
		return handle(fn, prefix, server)
	}

	wrapWithStreamParm := func(fn streamHandlerFunc, prefix string) http.Handler {
		return streamHandle(fn, prefix, driverHandler)
	}

	wrapWithParms := func(fn fileHandlerFunc, prefix string) http.Handler {
		return fileHandle(fn, prefix, driverHandler, server)
	}

	wrapWithPreviewParms := func(fn previewHandlerFunc, prefix string) http.Handler {
		return previewHandle(fn, prefix, driverHandler)
	}

	_ = wrapWithParms
	_ = wrapWithPreviewParms

	r.HandleFunc("/health", healthHandler)

	uploader := r.PathPrefix("/upload").Subrouter()

	uploader.PathPrefix("/upload-link").Handler(monkey(uploadLinkHandler, "/upload/upload-link")).Methods("GET")                    // recons done
	uploader.PathPrefix("/file-uploaded-bytes").Handler(monkey(uploadedBytesHandler, "/upload/file-uploaded-bytes")).Methods("GET") // recons done
	uploader.PathPrefix("/upload-link/{uid}").Handler(monkey(uploadChunksHandler, "/upload/upload-link")).Methods("POST")           // recons done

	api := r.PathPrefix("/api").Subrouter()

	api.PathPrefix("/nodes").Handler(common(nodesGetHandler)).Methods("GET")
	api.PathPrefix("/repos").Handler(common(reposGetHandler)).Methods("GET")

	// todo
	api.PathPrefix("/resources").Handler(wrapWithParms(listHandler, "/api/resources/")).Methods("GET")
	api.PathPrefix("/stream").Handler(wrapWithStreamParm(streamHandler, "/api/stream/")).Methods("GET")
	api.PathPrefix("/preview/{path:.*}").Handler(wrapWithPreviewParms(previewHandlerEx, "/api/preview/")).Methods("GET") // todo

	// api.PathPrefix("/resources").Handler(monkey(resourceGetHandler, "/api/resources")).Methods("GET")
	api.PathPrefix("/resources").Handler(monkey(resourcePostHandler, "/api/resources")).Methods("POST")             // create // recons done
	api.PathPrefix("/resources").Handler(monkey(batchDeleteHandler(fileCache), "/api/resources")).Methods("DELETE") // recons done

	api.PathPrefix("/resources").Handler(monkey(resourcePutHandler, "/api/resources")).Methods("PUT")                // edit txt // recons done
	api.PathPrefix("/resources").Handler(monkey(resourcePatchHandler(fileCache), "/api/resources")).Methods("PATCH") // todo rename // recons done

	api.PathPrefix("/mounted").Handler(monkey(resourceMountedHandler, "/api/mounted")).Methods("GET")  // no need to recons
	api.PathPrefix("/mount").Handler(monkey(resourceMountHandler, "/api/mount")).Methods("POST")       // no need to recons
	api.PathPrefix("/unmount").Handler(monkey(resourceUnmountHandler, "/api/unmount")).Methods("POST") // recons done
	// Because /api/resources/AppData is proxied under current arch, new api must be of a different prefix,
	// and try to access /api/resources/AppData in the handle func.
	api.PathPrefix("/paste").Handler(monkey(resourcePasteHandler(fileCache), "/api/paste")).Methods("PATCH") // paste // recons done
	api.PathPrefix("/task").Handler(monkey(resourceTaskGetHandler, "/api/task")).Methods("GET")              // no need to recons
	api.PathPrefix("/task").Handler(monkey(resourceTaskDeleteHandler, "/api/task")).Methods("DELETE")        // no need to recons

	api.PathPrefix("/share/shareable").Handler(monkey(shareableGetHandler, "/api/share/shareable")).Methods("GET")         // TODO: not used now, will be rewrite
	api.PathPrefix("/share/shareable").Handler(monkey(shareablePutHandler, "/api/share/shareable")).Methods("PUT")         // TODO: not used now, will be rewrite
	api.PathPrefix("/share/share_link").Handler(monkey(shareLinkGetHandler, "/api/share/share_link")).Methods("GET")       // TODO: not used now, will be rewrite
	api.PathPrefix("/share/share_link").Handler(monkey(shareLinkPostHandler, "/api/share/share_link")).Methods("POST")     // TODO: not used now, will be rewrite
	api.PathPrefix("/share/share_link").Handler(monkey(shareLinkDeleteHandler, "/api/share/share_link")).Methods("DELETE") // TODO: not used now, will be rewrite

	api.PathPrefix("/raw").Handler(monkey(rawHandler, "/api/raw")).Methods("GET")                         // recons done
	api.PathPrefix("/md5").Handler(monkey(md5Handler, "/api/md5")).Methods("GET")                         // recons done
	api.PathPrefix("/permission").Handler(monkey(permissionGetHandler, "/api/permission")).Methods("GET") // recons done
	api.PathPrefix("/permission").Handler(monkey(permissionPutHandler, "/api/permission")).Methods("PUT") // recons done

	api.PathPrefix("/smb_history").Handler(monkey(smbHistoryGetHandler, "/api/smb_history")).Methods("GET")       // no need to recons
	api.PathPrefix("/smb_history").Handler(monkey(smbHistoryPutHandler, "/api/smb_history")).Methods("PUT")       // no need to recons
	api.PathPrefix("/smb_history").Handler(monkey(smbHistoryDeleteHandler, "/api/smb_history")).Methods("DELETE") // no need to recons
	//api.PathPrefix("/smb_history").Handler(monkey(smbHistoryDeleteHandler, "/api/smb_history")).Methods("PATCH")	// recons delete this

	// api.PathPrefix("/preview/{size}/{path:.*}").
	// 	Handler(monkey(previewHandler(imgSvc, fileCache, server.EnableThumbnails, server.ResizePreview), "/api/preview")).Methods("GET")

	files := r.PathPrefix("/files").Subrouter()
	files.HandleFunc("/healthcheck", ginHandlerAdapter(rpc.RpcEngine))

	return stripPrefix(server.BaseURL, r), nil
}

func ginHandlerAdapter(ginEngine *gin.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		klog.Infoln("You see me~")
		klog.Infoln("request: ", r)
		klog.Infoln("request header: ", r.Header)
		klog.Infoln("request body: ", r.Body)
		ginEngine.ServeHTTP(w, r)
	}
}

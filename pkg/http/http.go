package http

import (
	"files/pkg/common"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/files"
	"files/pkg/preview"
	"files/pkg/rpc"
	"net/http"

	"github.com/gin-gonic/gin"
	"k8s.io/klog/v2"

	"github.com/gorilla/mux"
)

func NewHandler(
	imgSvc preview.ImgService,
	fileCache files.FileCache,
	server *common.Server,
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

	r.HandleFunc("/health", healthHandler)

	uploader := r.PathPrefix("/upload").Subrouter()

	uploader.PathPrefix("/upload-link").Handler(monkey(uploadLinkHandler, "/upload/upload-link")).Methods("GET")                    // recons done
	uploader.PathPrefix("/file-uploaded-bytes").Handler(monkey(uploadedBytesHandler, "/upload/file-uploaded-bytes")).Methods("GET") // recons done
	uploader.PathPrefix("/upload-link/{uid}").Handler(monkey(uploadChunksHandler, "/upload/upload-link")).Methods("POST")           // recons done

	api := r.PathPrefix("/api").Subrouter()

	api.PathPrefix("/nodes").Handler(common(nodesGetHandler)).Methods("GET")
	api.PathPrefix("/repos").Handler(common(reposGetHandler)).Methods("GET")
	api.PathPrefix("/repos").Handler(common(createRepoHandler)).Methods("POST")
	api.PathPrefix("/repos").Handler(common(deleteRepoHandler)).Methods("DELETE")
	api.PathPrefix("/repos").Handler(common(renameRepoHandler)).Methods("PATCH")

	api.PathPrefix("/resources").Handler(wrapperFilesResourcesArgs(listHandler, "/api/resources/")).Methods("GET")    // list files
	api.PathPrefix("/resources").Handler(wrapperFilesResourcesArgs(createHandler, "/api/resources/")).Methods("POST") // create directory
	api.PathPrefix("/resources").Handler(wrapperFilesResourcesArgs(renameHandler, "/api/resources")).Methods("PATCH") // rename
	api.PathPrefix("/resources").Handler(wrapperFilesDeleteArgs(deleteHandler, "/api/resources/")).Methods("DELETE")  // delete

	api.PathPrefix("/tree").Handler(wrapWithTreeParm(treeHandler, "/api/tree/")).Methods("GET")                      // walk through files
	api.PathPrefix("/preview/{path:.*}").Handler(wrapperPreviewArgs(previewHandler, "/api/preview/")).Methods("GET") // preview image
	api.PathPrefix("/raw").Handler(wrapperRawArgs(rawHandler, "/api/raw")).Methods("GET")
	// + paste+task todo
	api.PathPrefix("/paste").Handler(wrapperPasteArgs("/api/paste")).Methods("PATCH") // todo paste
	api.PathPrefix("/task").Handler(wrapperTaskArgs("/api/task")).Methods("GET")      // todo task
	api.PathPrefix("/task").Handler(wrapperTaskArgs("/api/task")).Methods("DELETE")   // todo task

	// ~

	api.PathPrefix("/resources").Handler(monkey(resourcePutHandler, "/api/resources")).Methods("PUT") // edit txt // recons done

	api.PathPrefix("/mounted").Handler(monkey(resourceMountedHandler, "/api/mounted")).Methods("GET")  // no need to recons
	api.PathPrefix("/mount").Handler(monkey(resourceMountHandler, "/api/mount")).Methods("POST")       // no need to recons
	api.PathPrefix("/unmount").Handler(monkey(resourceUnmountHandler, "/api/unmount")).Methods("POST") // recons done
	// Because /api/resources/AppData is proxied under current arch, new api must be of a different prefix,
	// and try to access /api/resources/AppData in the handle func.

	api.PathPrefix("/share/shareable").Handler(monkey(shareableGetHandler, "/api/share/shareable")).Methods("GET")         // TODO: not used now, will be rewrite
	api.PathPrefix("/share/shareable").Handler(monkey(shareablePutHandler, "/api/share/shareable")).Methods("PUT")         // TODO: not used now, will be rewrite
	api.PathPrefix("/share/share_link").Handler(monkey(shareLinkGetHandler, "/api/share/share_link")).Methods("GET")       // TODO: not used now, will be rewrite
	api.PathPrefix("/share/share_link").Handler(monkey(shareLinkPostHandler, "/api/share/share_link")).Methods("POST")     // TODO: not used now, will be rewrite
	api.PathPrefix("/share/share_link").Handler(monkey(shareLinkDeleteHandler, "/api/share/share_link")).Methods("DELETE") // TODO: not used now, will be rewrite

	api.PathPrefix("/md5").Handler(monkey(md5Handler, "/api/md5")).Methods("GET")                         // recons done
	api.PathPrefix("/permission").Handler(monkey(permissionGetHandler, "/api/permission")).Methods("GET") // recons done
	api.PathPrefix("/permission").Handler(monkey(permissionPutHandler, "/api/permission")).Methods("PUT") // recons done

	api.PathPrefix("/smb_history").Handler(monkey(smbHistoryGetHandler, "/api/smb_history")).Methods("GET")       // no need to recons
	api.PathPrefix("/smb_history").Handler(monkey(smbHistoryPutHandler, "/api/smb_history")).Methods("PUT")       // no need to recons
	api.PathPrefix("/smb_history").Handler(monkey(smbHistoryDeleteHandler, "/api/smb_history")).Methods("DELETE") // no need to recons
	//api.PathPrefix("/smb_history").Handler(monkey(smbHistoryDeleteHandler, "/api/smb_history")).Methods("PATCH")	// recons delete this

	files := r.PathPrefix("/files").Subrouter()
	files.HandleFunc("/healthcheck", ginHandlerAdapter(rpc.RpcEngine))

	// for temp test and data monitoring
	api.PathPrefix("/seahub/users").Handler(monkey(seahub.SeahubUsersGetHandler, "/api/seahub/users")).Methods("GET")
	api.PathPrefix("/seahub/upload/upload-link").Handler(monkey(seahub.HandleUploadLink, "/api/seahub/upload/upload-link")).Methods("GET")                    // recons done
	api.PathPrefix("/seahub/upload/file-uploaded-bytes").Handler(monkey(seahub.HandleUploadedBytes, "/api/seahub/upload/file-uploaded-bytes")).Methods("GET") //

	callback := r.PathPrefix("/callback").Subrouter()
	callback.Path("/create").Handler(monkey(seahub.CallbackCreateHandler, "/callback/create")).Methods("POST")
	callback.Path("/delete").Handler(monkey(seahub.CallbackDeleteHandler, "/callback/delete")).Methods("POST")

	// will delete after test
	callbackTest := api.PathPrefix("/callback").Subrouter()
	callbackTest.Path("/create").Handler(monkey(seahub.CallbackCreateHandler, "/callback/create")).Methods("POST")
	callbackTest.Path("/delete").Handler(monkey(seahub.CallbackDeleteHandler, "/callback/delete")).Methods("POST")

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

package http

import (
	"fmt"
	"github.com/filebrowser/filebrowser/v2/rpc"
	"github.com/gin-gonic/gin"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/storage"
)

type modifyRequest struct {
	What  string   `json:"what"`  // Answer to: what data type?
	Which []string `json:"which"` // Answer to: which fields?
}

func NewHandler(
	imgSvc ImgService,
	fileCache FileCache,
	store *storage.Storage,
	server *settings.Server,
	// assetsFs fs.FS,
) (http.Handler, error) {
	server.Clean()

	r := mux.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Security-Policy", `default-src 'self'; style-src 'unsafe-inline';`)
			next.ServeHTTP(w, r)
		})
	})
	//index, static := getStaticHandlers(store, server, assetsFs)

	// NOTE: This fixes the issue where it would redirect if people did not put a
	// trailing slash in the end. I hate this decision since this allows some awful
	// URLs https://www.gorillatoolkit.org/pkg/mux#Router.SkipClean
	r = r.SkipClean(true)

	monkey := func(fn handleFunc, prefix string) http.Handler {
		return handle(fn, prefix, store, server)
	}

	r.HandleFunc("/health", healthHandler)
	// r.PathPrefix("/static").Handler(static)
	// r.NotFoundHandler = index

	api := r.PathPrefix("/api").Subrouter()

	api.Handle("/login", monkey(loginHandler, ""))
	api.Handle("/signup", monkey(signupHandler, ""))
	api.Handle("/renew", monkey(renewHandler, ""))

	users := api.PathPrefix("/users").Subrouter()
	users.Handle("", monkey(usersGetHandler, "")).Methods("GET")
	users.Handle("", monkey(userPostHandler, "")).Methods("POST")
	users.Handle("/{id:[0-9]+}", monkey(userPutHandler, "")).Methods("PUT")
	users.Handle("/{id:[0-9]+}", monkey(userGetHandler, "")).Methods("GET")
	users.Handle("/{id:[0-9]+}", monkey(userDeleteHandler, "")).Methods("DELETE")

	api.PathPrefix("/resources").Handler(monkey(resourceGetHandler, "/api/resources")).Methods("GET")
	api.PathPrefix("/resources").Handler(monkey(resourceDeleteHandler(fileCache), "/api/resources")).Methods("DELETE")
	api.PathPrefix("/resources").Handler(monkey(resourcePostHandler(fileCache), "/api/resources")).Methods("POST")
	api.PathPrefix("/resources").Handler(monkey(resourcePutHandler, "/api/resources")).Methods("PUT")
	api.PathPrefix("/resources").Handler(monkey(resourcePatchHandler(fileCache), "/api/resources")).Methods("PATCH")
	// Because /api/resources/AppData is proxied under current arch, new api must be of a different prefix,
	// and try to access /api/resources/AppData in the handle func.
	api.PathPrefix("/paste").Handler(monkey(resourcePasteHandler(fileCache), "/api/paste")).Methods("PATCH")

	api.PathPrefix("/usage").Handler(monkey(diskUsage, "/api/usage")).Methods("GET")

	api.Path("/shares").Handler(monkey(shareListHandler, "/api/shares")).Methods("GET")
	api.PathPrefix("/share").Handler(monkey(shareGetsHandler, "/api/share")).Methods("GET")
	api.PathPrefix("/share").Handler(monkey(sharePostHandler, "/api/share")).Methods("POST")
	api.PathPrefix("/share").Handler(monkey(shareDeleteHandler, "/api/share")).Methods("DELETE")

	api.Handle("/settings", monkey(settingsGetHandler, "")).Methods("GET")
	api.Handle("/settings", monkey(settingsPutHandler, "")).Methods("PUT")

	api.PathPrefix("/raw").Handler(monkey(rawHandler, "/api/raw")).Methods("GET")
	api.PathPrefix("/preview/{size}/{path:.*}").
		Handler(monkey(previewHandler(imgSvc, fileCache, server.EnableThumbnails, server.ResizePreview), "/api/preview")).Methods("GET")
	api.PathPrefix("/command").Handler(monkey(commandsHandler, "/api/command")).Methods("GET")
	api.PathPrefix("/search").Handler(monkey(searchHandler, "/api/search")).Methods("GET")
	if rpc.KnowledgeBase == "True" {
		api.HandleFunc("/get_dataset_folder_status_test", ginHandlerAdapter(rpc.RpcEngine))
		api.HandleFunc("/update_dataset_folder_paths_test", ginHandlerAdapter(rpc.RpcEngine))
	}

	files := r.PathPrefix("/files").Subrouter()
	files.HandleFunc("/healthcheck", ginHandlerAdapter(rpc.RpcEngine))
	//files.HandleFunc("/input", ginHandlerAdapter(rpc.RpcEngine))
	//files.HandleFunc("/delete", ginHandlerAdapter(rpc.RpcEngine))
	files.HandleFunc("/query", ginHandlerAdapter(rpc.RpcEngine))

	provider := r.PathPrefix("/provider").Subrouter()
	provider.HandleFunc("/query_file", ginHandlerAdapter(rpc.RpcEngine))
	provider.HandleFunc("/get_search_folder_status", ginHandlerAdapter(rpc.RpcEngine))
	provider.HandleFunc("/update_search_folder_paths", ginHandlerAdapter(rpc.RpcEngine))
	provider.HandleFunc("/get_dataset_folder_status", ginHandlerAdapter(rpc.RpcEngine))
	provider.HandleFunc("/update_dataset_folder_paths", ginHandlerAdapter(rpc.RpcEngine))

	public := api.PathPrefix("/public").Subrouter()
	public.PathPrefix("/dl").Handler(monkey(publicDlHandler, "/api/public/dl/")).Methods("GET")
	//public.PathPrefix("/share").Handler(monkey(publicShareHandler, "/api/public/share/")).Methods("GET")

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

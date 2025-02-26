package auth

import (
	"github.com/filebrowser/filebrowser/v2/settings"
	"github.com/filebrowser/filebrowser/v2/users"
	"net/http"
)

// Auther is the authentication interface.
type Auther interface {
	// Auth is called to authenticate a request.
	Auth(r *http.Request, usr users.Store, stg *settings.Settings, srv *settings.Server) (*users.User, error)
	// LoginPage indicates if this auther needs a login page.
	LoginPage() bool
}

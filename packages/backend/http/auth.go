package http

import (
	"net/http"
)

func withUser(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		d.user, _ = d.store.Users.Get(d.server.Root, uint(1))
		return fn(w, r, d)
	}
}

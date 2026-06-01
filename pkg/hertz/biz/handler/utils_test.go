package handler

import (
	"context"
	"net/http"
	"testing"

	"files/pkg/common"
	"files/pkg/models"

	"github.com/cloudwego/hertz/pkg/app"
)

// newReqCtx builds a minimal RequestContext with the given query string and
// optional internal-share-token header.
func newReqCtx(query, token string) *app.RequestContext {
	c := app.NewContext(0)
	uri := "/api/resources/drive/Home/a.txt"
	if query != "" {
		uri += "?" + query
	}
	c.Request.SetRequestURI(uri)
	c.Request.Header.SetMethod(http.MethodGet)
	if token != "" {
		c.Request.Header.Set(common.HeaderInternalShareToken, token)
	}
	return c
}

func TestRequireInternalShareToken(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		c := newReqCtx("", common.InternalShareToken())
		if !RequireInternalShareToken(c, "test") {
			t.Fatalf("valid token: got false, want true")
		}
	})

	t.Run("missing token", func(t *testing.T) {
		c := newReqCtx("", "")
		if RequireInternalShareToken(c, "test") {
			t.Fatalf("missing token: got true, want false")
		}
		if c.Response.StatusCode() != http.StatusForbidden {
			t.Fatalf("missing token: status = %d, want 403", c.Response.StatusCode())
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		c := newReqCtx("", "not-the-token")
		if RequireInternalShareToken(c, "test") {
			t.Fatalf("wrong token: got true, want false")
		}
		if c.Response.StatusCode() != http.StatusForbidden {
			t.Fatalf("wrong token: status = %d, want 403", c.Response.StatusCode())
		}
	})
}

func TestGate_ShareShortCircuit(t *testing.T) {
	fp := &models.FileParam{Owner: "alice", FileType: "drive", Extend: "Home", Path: "/a.txt"}

	t.Run("share=1 valid token allows", func(t *testing.T) {
		c := newReqCtx("share=1", common.InternalShareToken())
		if !Gate(context.Background(), c, fp, models.ActionRead, true, "test") {
			t.Fatalf("share=1 valid token: got false, want true")
		}
	})

	t.Run("share=1 invalid token denied", func(t *testing.T) {
		c := newReqCtx("share=1", "")
		if Gate(context.Background(), c, fp, models.ActionRead, true, "test") {
			t.Fatalf("share=1 invalid token: got true, want false")
		}
		if c.Response.StatusCode() != http.StatusForbidden {
			t.Fatalf("share=1 invalid token: status = %d, want 403", c.Response.StatusCode())
		}
	})

	t.Run("skipShare=false ignores share=1 token branch", func(t *testing.T) {
		// With skipShare=false the share branch is not taken; Gate proceeds
		// to CheckAccessParam. We only assert it does NOT short-circuit to
		// true via the token path (CheckAccessParam denies without globals).
		c := newReqCtx("share=1", common.InternalShareToken())
		if Gate(context.Background(), c, fp, models.ActionRead, false, "test") {
			t.Fatalf("skipShare=false: got true, want false (must not use token short-circuit)")
		}
	})
}

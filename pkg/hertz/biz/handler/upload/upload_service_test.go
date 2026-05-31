package upload

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"files/pkg/common"
	"files/pkg/drivers/base"
	"files/pkg/drivers/sync/seahub"
	"files/pkg/models"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

// fakeExecute embeds base.Execute (nil) so it satisfies the interface; only
// UploadLink is overridden to return the configured error.
type fakeExecute struct {
	base.Execute
	uploadLinkErr error
}

func (f *fakeExecute) UploadLink(*models.FileUploadArgs) ([]byte, error) {
	if f.uploadLinkErr != nil {
		return nil, f.uploadLinkErr
	}
	return []byte("ok"), nil
}

// newUploadLinkCtx builds a share=1 upload-link request carrying a valid
// internal token so the handler bypasses CheckAccess and reaches the driver.
func newUploadLinkCtx(token string) *app.RequestContext {
	c := app.NewContext(0)
	c.Request.SetRequestURI("/upload/upload_link?file_path=/drive/Home/a.txt&from=web&share=1")
	c.Request.Header.SetMethod(http.MethodGet)
	c.Request.Header.Set(common.REQUEST_HEADER_OWNER, "alice")
	if token != "" {
		c.Request.Header.Set(common.HeaderInternalShareToken, token)
	}
	return c
}

func TestUploadLinkMethod_ErrorMapping(t *testing.T) {
	saved := newFileHandler
	t.Cleanup(func() { newFileHandler = saved })

	cases := []struct {
		name       string
		uploadErr  error
		wantStatus int
	}{
		{"sync permission denied -> 403", seahub.ErrSyncPermissionDenied, consts.StatusForbidden},
		{"generic error -> 500", errors.New("boom"), consts.StatusInternalServerError},
		{"success -> 200", nil, consts.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newFileHandler = func(string, *base.HandlerParam) base.Execute {
				return &fakeExecute{uploadLinkErr: tc.uploadErr}
			}
			c := newUploadLinkCtx(common.InternalShareToken())
			UploadLinkMethod(context.Background(), c)
			if got := c.Response.StatusCode(); got != tc.wantStatus {
				t.Fatalf("status = %d, want %d", got, tc.wantStatus)
			}
		})
	}
}

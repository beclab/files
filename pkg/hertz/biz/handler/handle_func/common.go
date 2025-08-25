package handle_func

import (
	"bytes"
	"context"
	"files/pkg/common"
	"files/pkg/models"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"reflect"
	"strings"
)

type commonFunc func(contextQueryArgs *models.QueryParam) ([]byte, error)

func CommonHandle(ctx context.Context, c *app.RequestContext, req interface{}, fn commonFunc) []byte {
	var path = c.Path()
	var owner = string(c.GetHeader(common.REQUEST_HEADER_OWNER))
	if owner == "" {
		c.AbortWithStatusJSON(consts.StatusBadRequest, utils.H{"error": "user not found"})
		return nil
	}

	var contextQueryArgs = CreateQueryParam(owner, ctx, c, false, false)

	klog.Infof("Incoming Path: %s, user: %s, method: %s", path, owner, c.Method())

	res, err := fn(contextQueryArgs)
	c.SetContentType("application/json")

	if err != nil {
		c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{
			"code":    1,
			"message": err.Error(),
		})
		return nil
	}

	return res
}

func CreateQueryParam(owner string, ctx context.Context, c *app.RequestContext, enableThumbnails bool, resizePreview bool) *models.QueryParam {
	// TODO: add for sync
	sizeStr := c.Query("size")
	if sizeStr == "" {
		sizeStr = c.Query("thumb")
	}
	// add end

	header := make(http.Header)
	c.Request.Header.VisitAll(func(key, value []byte) {
		headerKey := string(key)
		headerValue := string(value)
		header.Add(headerKey, headerValue)
	})

	return &models.QueryParam{
		Ctx:                     ctx,
		Owner:                   owner,
		PreviewSize:             sizeStr,
		PreviewEnableThumbnails: enableThumbnails, // todo
		PreviewResizePreview:    resizePreview,    // todo
		RawInline:               strings.TrimSpace(c.Query("inline")),
		RawMeta:                 strings.TrimSpace(c.Query("meta")),
		Files:                   strings.TrimSpace(c.Query("files")),
		FileMode:                strings.TrimSpace(c.Query("mode")),
		RepoName:                strings.TrimSpace(c.Query("repoName")),
		RepoId:                  strings.TrimSpace(c.Query("repoId")),
		Destination:             strings.TrimSpace(c.Query("destination")),
		ShareType:               strings.TrimSpace(c.Query("type")), // "mine", "shared", "share_to_me"
		Header:                  header,
		Body:                    io.NopCloser(bytes.NewReader(c.Request.Body())),
	}
}

type handleFunc func(c *app.RequestContext, req interface{}, prefix string) ([]byte, int, error)

func MonkeyHandle(ctx context.Context, c *app.RequestContext, req interface{}, fn handleFunc, prefix string) []byte {
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")

	res, status, err := fn(c, req, prefix)

	if status >= 400 || err != nil {
		clientIP := c.ClientIP()
		klog.Errorf("%s: %d %s %v", c.Request.URI().Path(), status, clientIP, err)
	}

	if status != 0 && status != consts.StatusOK {
		if status >= consts.StatusBadRequest {
			txt := http.StatusText(status)
			if err != nil {
				txt = err.Error()
			}

			c.Header("Content-Type", "application/json")
			c.Status(status)
			c.JSON(consts.StatusOK, utils.H{
				"code":    1,
				"message": txt,
			})
		} else {
			txt := http.StatusText(status)
			c.String(status, "%d %s", status, txt)
		}
		return nil
	}

	return res
}

func Coalesce(vals ...interface{}) interface{} {
	for _, v := range vals {
		if val := reflect.ValueOf(v); !isNil(val) {
			if val.Kind() != reflect.Ptr && val.Kind() != reflect.Interface {
				return v
			}
			if !val.IsNil() {
				return v
			}
		}
	}
	return nil
}

func isNil(val reflect.Value) bool {
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func:
		return val.IsNil()
	default:
		return false
	}
}

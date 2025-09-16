package router

import (
	"context"
	"files/pkg/common"
	"files/pkg/hertz/biz/dal/database"
	"files/pkg/models"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"k8s.io/klog/v2"
)

func TimingMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()

		path := c.Path()

		klog.Infof("%s %s starts at %v", string(c.Method()), path, start.Format("2006-01-02 15:04:05"))

		defer func() {
			elapsed := time.Since(start)
			klog.Infof("%s %s execution time: %v", string(c.Method()), path, elapsed)
		}()

		c.Next(ctx)
	}
}

func CookieMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		bflName := string(c.GetHeader("X-Bfl-User"))
		newCookie := string(c.GetHeader("Cookie"))

		if bflName != "" {
			oldCookie := common.BflCookieCache[bflName]
			if newCookie != oldCookie {
				common.BflCookieCache[bflName] = newCookie
			}
		}

		klog.Infof("BflCookieCache= %v", common.BflCookieCache)
		c.Next(ctx)
	}
}

func ShareMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var bflName = string(c.GetHeader("X-Bfl-User"))
		var method = string(c.Request.Method())
		var path = string(c.Request.Path())
		if !strings.HasPrefix(path, "/api/share/") {
			c.Next(ctx)
			return
		}

		var preview, raw bool
		var download, upload bool
		_ = download
		_ = upload

		var paths = strings.TrimPrefix(path, "/api/share")

		if strings.HasPrefix(paths, "/preview/") {
			preview = true
			paths = strings.TrimPrefix(paths, "/preview")
		} else if strings.HasPrefix(paths, "/raw/") {
			raw = true
			paths = strings.TrimPrefix(paths, "/raw/")
		} else if strings.HasPrefix(paths, "/upload/") {
			upload = true
		}

		var shareParam, err = models.CreateFileParam(bflName, paths)
		if err != nil {
			c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": err.Error()})
			return
		}

		klog.Infof("share params: %s", common.ToJson(shareParam))

		shared, err := database.QueryShareById(shareParam.Extend)
		if err != nil {
			klog.Errorf("postgres.QueryShareById error: %v", err)
			c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": err.Error()})
			return
		}

		if !((shared.FileType == common.Drive && shared.Extend == common.Home) || shared.FileType == common.Sync) {
			klog.Errorf("share path invalid, fileType: %s, extend: %s", shared.FileType, shared.Extend)
			c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "fileType invalid"})
			return
		}

		// todo check expired

		var sharedPath = shared.Path
		var shareType = strings.ToLower(shared.ShareType)
		var sharePermission = shared.Permission
		var shareOwner = shared.Owner

		_ = sharePermission
		_ = shareOwner

		if shareType == common.ShareTypeInternal {
			shareMember, err := database.QueryShareMemberById(shared.ID)
			if err != nil {
				klog.Errorf("postgres.QueryShareMemberById error: %v", err)
				c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": err.Error()})
				return
			}

			if shareMember.ShareMember == "" {
				klog.Error("shareMember is nil")
				c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "shareMember is nil"})
				return
			}

			var matchedMember bool
			var members = strings.Split(shareMember.ShareMember, ",")
			for _, m := range members {
				if m == bflName {
					matchedMember = true
					break
				}
			}

			if !matchedMember {
				klog.Error("shareMember is nil")
				c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "shareMember is nil"})
				return
			}

		}

		if permit := checkPermission(shared.ShareType, shared.Permission, method, preview, raw, download, upload); !permit {
			c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "No permission"})
			return
		}

		var pathRewrite string
		if shareParam.Path != "/" {
			pathRewrite = strings.TrimRight(sharedPath, "/") + shareParam.Path
		} else {
			pathRewrite = sharedPath
		}

		var host = fmt.Sprintf("http://%s", common.SERVER_HOST)
		var query = string(c.Request.QueryString())
		pathRewrite = filepath.Join(shared.FileType, shared.Extend) + pathRewrite
		var rewritePrefix string
		if preview {
			rewritePrefix = "/api/preview/"
		} else if raw {
			rewritePrefix = "/api/raw/"
		} else if upload {
			// todo upload
			// rewritePrefix = ""
		} else {
			rewritePrefix = "/api/resources/"
		}

		var url = host + rewritePrefix + pathRewrite
		if query != "" {
			url = url + "?" + query
		}

		klog.Infof("share rewrite url: %s", url)

		req, _ := http.NewRequest(string(c.Request.Method()), url, nil)

		c.Request.Header.VisitAll(func(key, value []byte) {
			req.Header.Set(string(key), string(value))
		})
		req.Header.Set(common.REQUEST_HEADER_OWNER, shareOwner)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.String(502, "proxy error: %v", err)
			return
		}
		defer resp.Body.Close()

		for k, vv := range resp.Header {
			for _, v := range vv {
				c.Header(k, v)
			}
		}
		c.Status(resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		c.Write(body)
	}
}

func checkPermission(shareType string, permission int32, method string, preview, raw bool, download, upload bool) bool {
	/**
	 * permission
	 * 0 - no permit
	 * 1 - view
	 * 2 - upload only
	 * 3 - edit
	 * 4 - admin
	 */

	if shareType == common.ShareTypeInternal {
		return true
	}

	switch permission {
	case 1:
		return method == http.MethodGet && !upload
	case 2:
		return false
	case 3:
		return false
	case 4:
		return true
	default:
		return false
	}
}

package router

import (
	"bytes"
	"context"
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/hertz/biz/dal/database"
	"files/pkg/hertz/biz/model/api/share"
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

type ShareAccess struct {
	Method   string
	Resource bool
	Preview  bool
	Raw      bool
	Download bool
	Upload   bool
}

var (
	ShareHostPrefix        = "share."
	ShareApiResourcesPath  = "/api/resources"
	ShareApiPreviewPath    = "/api/preview"
	ShareApiRawPath        = "/api/raw"
	ShareApiUploadPath     = "/upload"
	ShareApiUploadLinkPath = "/upload/upload-link"
	ShareApiUploadedPath   = "/upload/file-uploaded-bytes"
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

// + share
func ShareMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var bflName = string(c.GetHeader("X-Bfl-User"))
		var method = string(c.Request.Method())
		var host = string(c.Request.Host())
		var path = string(c.Request.Path())

		var paramPath string
		var urlShareType string
		var shareAccess = &ShareAccess{
			Method: method,
		}

		if strings.HasPrefix(host, ShareHostPrefix) {
			urlShareType = common.ShareTypeExternal
		} else {
			urlShareType = common.ShareTypeInternal
		}

		if strings.HasPrefix(path, ShareApiResourcesPath) {
			paramPath = strings.TrimPrefix(path, ShareApiResourcesPath)
			shareAccess.Resource = true
		} else if strings.HasPrefix(path, ShareApiPreviewPath) {
			paramPath = strings.TrimPrefix(path, ShareApiPreviewPath)
			shareAccess.Preview = true
		} else if strings.HasPrefix(path, ShareApiRawPath) {
			paramPath = strings.TrimPrefix(path, ShareApiRawPath)
			if c.Query("inline") == "true" {
				shareAccess.Raw = true
			} else {
				shareAccess.Download = true
			}
		} else if strings.HasPrefix(path, ShareApiUploadPath) {
			shareAccess.Upload = true
			if strings.HasPrefix(path, ShareApiUploadLinkPath) {
				if method == http.MethodGet {
					paramPath = c.Query("file_path")
				} else {
					paramPath = c.Query("file_path") // todo body.parent_id
				}
			} else if strings.HasPrefix(path, ShareApiUploadedPath) {
				paramPath = c.Query("parent_dir")
			}
		}

		shareParam, _ := models.CreateFileParam(bflName, paramPath)

		if shareParam == nil || shareParam.FileType != common.Share {
			c.Next(ctx)
			return
		}

		klog.Infof("[share] share params: %s", common.ToJson(shareParam))

		var uploadLink, uploadBytes, uploadChunk bool
		var uploadLinkId, uploadParentDir, uploadFileName string
		_ = uploadParentDir

		shared, err := checkSharePath(shareParam.Extend)
		if err != nil {
			c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": err.Error()})
			return
		}

		if urlShareType != shared.ShareType {
			klog.Errorf("url.shareType %s not equal sharePaths.shareType %s", urlShareType, shared.ShareType)
			c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "url invalid"})
			return
		}

		klog.Infof("[share] share path: %s", common.ToJson(shared))

		var sharedPath = shared.Path
		var shareType = strings.ToLower(shared.ShareType)
		var sharePermission = shared.Permission
		var shareOwner = shared.Owner

		_ = sharePermission
		_ = shareOwner

		if shareType == common.ShareTypeInternal {
			if err = checkInternal(bflName, shared, shareAccess); err != nil {
				klog.Errorf("check internal error: %v", err)
				c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "No permission"})
				return
			}
		} else if shareType == common.ShareTypeExternal {
			if err = checkExternal(bflName, c.Query("token"), shared, shareAccess); err != nil {
				klog.Errorf("check external error: %v", err)
				c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "No permission"})
				return
			}
		} else if shareType == common.ShareTypeSMB {

		} else {
			c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "No permission"})
			return
		}

		var pathRewrite string
		if shareParam.Path != "/" {
			pathRewrite = strings.TrimRight(sharedPath, "/") + shareParam.Path
		} else {
			pathRewrite = sharedPath
		}

		var redirect = fmt.Sprintf("http://%s", common.SERVER_HOST)
		var query = string(c.Request.QueryString())
		pathRewrite = filepath.Join(shared.FileType, shared.Extend) + pathRewrite
		var rewritePrefix string
		if shareAccess.Resource {
			rewritePrefix = ShareApiResourcesPath
		} else if shareAccess.Preview {
			rewritePrefix = ShareApiPreviewPath
		} else if shareAccess.Raw || shareAccess.Download {
			rewritePrefix = ShareApiRawPath
		} else if shareAccess.Upload {
			rewritePrefix = ShareApiUploadLinkPath + global.GlobalNode.GetMasterNode()
		} else {
			return
		}

		if !strings.HasSuffix(rewritePrefix, "/") {
			rewritePrefix += "/"
		}

		var url string
		if shareAccess.Upload { // upload
			shareOwner = bflName

			if uploadLink {
				url = redirect + rewritePrefix + "?file_path=" + "/" + pathRewrite + "&from=" + c.Query("from")
			} else if uploadBytes {
				url = redirect + rewritePrefix + "?parent_dir=" + "/" + pathRewrite + "&file_name=" + uploadFileName
			} else if uploadChunk {
				url = redirect + rewritePrefix + uploadLinkId + "?ret-json=" + c.Query("ret-json")
			}
		} else {
			url = redirect + rewritePrefix + pathRewrite
			if query != "" {
				url = url + "?" + query
			}
		}

		klog.Infof("[share] share rewrite url: %s, rewrite user: %s", url, shareOwner)

		req, _ := http.NewRequest(string(c.Request.Method()), url, nil)

		c.Request.Header.VisitAll(func(key, value []byte) {
			req.Header.Set(string(key), string(value))
		})
		req.Header.Set(common.REQUEST_HEADER_OWNER, shareOwner)

		body, err := io.ReadAll(c.Request.BodyStream())
		if err != nil {
			c.String(500, "failed to read request body: %v", err)
			return
		}

		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))

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
		bodyRes, _ := io.ReadAll(resp.Body)
		c.Write(bodyRes)
		c.Abort()
		return
	}
}

func checkSharePath(shareId string) (*share.SharePath, error) {
	sharePath, err := database.QueryShareById(shareId)
	if err != nil {
		klog.Errorf("postgres.QueryShareById error: %v", err)
		return nil, err
	}

	if sharePath == nil {
		return nil, errors.New("url invalid")
	}

	if !((sharePath.FileType == common.Drive && sharePath.Extend == common.Home) || sharePath.FileType == common.Sync) {
		klog.Errorf("share path invalid, type: %s, extend: %s", sharePath.FileType, sharePath.Extend)
		return nil, errors.New("url invalid")
	}

	return sharePath, nil
}

func checkInternal(currentOwner string, sharePaths *share.SharePath, shareAccess *ShareAccess) error {
	shareMember, err := database.QueryShareMemberById(sharePaths.ID)
	if err != nil {
		return fmt.Errorf("postgres.QueryShareMemberById error: %v", err)
	}

	if shareMember.ShareMember == "" {
		return errors.New("shareMember not found")
	}

	var matchedMember bool
	var members = strings.Split(shareMember.ShareMember, ",")
	for _, m := range members {
		if m == currentOwner {
			matchedMember = true
			break
		}
	}

	if !matchedMember {
		return errors.New("matchedMember is nil")
	}

	if permit := checkPermission(currentOwner, sharePaths.Owner, sharePaths.ShareType, sharePaths.Permission, shareAccess); !permit {
		return errors.New("authorization check failed")
	}
	return nil
}

func checkExternal(currentUser string, token string, sharePaths *share.SharePath, shareAccess *ShareAccess) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("token is nil")
	}

	shareToken, err := database.QueryShareExternalById(sharePaths.ID)
	if err != nil {
		return fmt.Errorf("postgres.QueryShareExternalById error: %v", err)
	}

	if shareToken == nil {
		return errors.New("shareToken not found")
	}

	if shareToken.Token != token {
		return fmt.Errorf("token invalid, shareToken.Token: %s", shareToken.Token)
	}

	expired, err := time.Parse("2006-01-02 15:04:05", shareToken.ExpireAt)
	if err != nil {
		return fmt.Errorf("shareToken expireAt invalid, shareToken.ExpireAt: %s", shareToken.Token)
	}

	if time.Now().After(expired) {
		return fmt.Errorf("shareToken expired, shareToken.ExpireAt: %s", shareToken.ExpireAt)
	}

	if permit := checkPermission(currentUser, sharePaths.Owner, sharePaths.ShareType, sharePaths.Permission, shareAccess); !permit {
		return errors.New("authorization check failed")
	}

	return nil

}

// method string, preview, raw bool, download, upload bool
func checkPermission(currentUser string, shareOwner string, shareType string, permission int32, shareAccess *ShareAccess) bool {
	/**
	 * permission
	 * 0 - no permit
	 * 1 - view, download
	 * 2 - upload only (external only)
	 * 3 - upload, download
	 * 4 - admin
	 */

	if currentUser == shareOwner {
		return true
	}

	switch permission {
	case 1:
		return shareAccess.Method == http.MethodGet && !shareAccess.Upload
	case 2: // only upload
		if shareType == common.ShareTypeExternal {
			return shareAccess.Upload
		}
		return false
	case 3:
		return (shareAccess.Resource && shareAccess.Method == http.MethodGet) || shareAccess.Upload || shareAccess.Download
	case 4:
		return true
	default:
		return false
	}
}

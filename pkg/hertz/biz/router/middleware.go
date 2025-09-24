package router

import (
	"bytes"
	"context"
	"errors"
	"files/pkg/common"
	"files/pkg/files"
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
		var err error
		var bflName = string(c.GetHeader("X-Bfl-User")) // todo if sharing externally, you may use the name from the host here.
		// var cookie = string(c.GetHeader("Cookie")) // todo for external sharing, this field may be empty unless it contains the sharer's cookie.
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

		var uploadLink, uploadBytes, uploadChunk bool
		var uploadLinkId, uploadParentDir, uploadFileName string
		_ = uploadParentDir

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
		} else if strings.HasPrefix(path, ShareApiUploadPath) { // ~ upload
			shareAccess.Upload = true
			if strings.HasPrefix(path, ShareApiUploadLinkPath) {
				if method == http.MethodGet {
					paramPath = c.Query("file_path")
					uploadLink = true

				} else {
					var tPath = strings.TrimPrefix(path, ShareApiUploadLinkPath)
					var tPos = strings.LastIndex(tPath, "/")
					if tPos == 0 {
						c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "upload-link path invalid"})
						return
					}
					uploadLinkId = tPath[tPos+1:]
					if uploadLinkId == "" {
						c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "upload-link path invalid"})
						return
					}

					uploadChunk = true
				}
			} else if strings.HasPrefix(path, ShareApiUploadedPath) {
				paramPath = c.Query("parent_dir")
				uploadFileName = c.Query("file_name")
				uploadBytes = true

			}
		}

		shareParam, _ := models.CreateFileParam(bflName, paramPath)

		if shareParam == nil || shareParam.FileType != common.Share {
			c.Next(ctx)
			return
		}

		klog.Infof("[share] share params: %s", common.ToJson(shareParam))
		if strings.HasPrefix(path, "/api/share/share_path/") ||
			strings.HasPrefix(path, "/api/share/share_token/") ||
			strings.HasPrefix(path, "/api/share/share_member/") {
			c.Next(ctx)
			return
		}

		var download, upload bool
		_ = download
		_ = upload

		shared, err := checkSharePath(shareParam.Extend)
		if err != nil {
			klog.Errorf("check sharePath error: %v", err)
			c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "No permission"})
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
		var shareBy = shared.Owner

		if shareType == common.ShareTypeInternal {
			if bflName != shared.Owner {
				if err = checkInternal(bflName, shared, shareAccess); err != nil {
					klog.Errorf("check internal error: %v", err)
					c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "No permission"})
					return
				}
			}
		} else if shareType == common.ShareTypeExternal {
			var token = c.Query("token")
			if err = checkExternal(bflName, token, shared, shareAccess); err != nil {
				klog.Errorf("check external error: %v", err)
				c.AbortWithStatusJSON(consts.StatusInternalServerError, utils.H{"error": "No permission"})
				return
			} else {
				if ok := accessIntercept(c, shared, shareType, shareAccess); ok {
					return
				}
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
			if uploadBytes {
				rewritePrefix = ShareApiUploadedPath + "/" + global.GlobalNode.GetMasterNode()
			} else {
				rewritePrefix = ShareApiUploadLinkPath + "/" + global.GlobalNode.GetMasterNode()
			}

		} else {
			return
		}

		if !strings.HasSuffix(rewritePrefix, "/") {
			rewritePrefix += "/"
		}

		var url string
		var accessOwner string
		if shareAccess.Upload { // + upload
			if shareType == common.ShareTypeExternal {
				accessOwner = shareBy
			} else {
				accessOwner = bflName
			}

			if uploadLink {
				url = redirect + rewritePrefix + "?file_path=" + "/" + pathRewrite + "&from=" + c.Query("from") + "&share=1&sharetype=" + shareType
			} else if uploadBytes {
				url = redirect + rewritePrefix + "?parent_dir=" + "/" + pathRewrite + "&file_name=" + uploadFileName + "&share=1&sharetype=" + shareType
			} else if uploadChunk {
				url = redirect + rewritePrefix + uploadLinkId + "?&ret-json=" + c.Query("ret-json") + "&share=1&sharetype=" + shareType + "&shareby=" + shareBy + "&shareby_path=" + "/" + pathRewrite
			}
		} else {
			accessOwner = shareBy

			url = redirect + rewritePrefix + pathRewrite
			if query != "" {
				url = url + "?" + query + "&share=1&sharetype=" + shareType
			} else {
				url = url + "?share=1&sharetype=" + shareType
			}
		}

		// +
		klog.Infof("[share] share rewrite url: %s, access: %s, shareby: %s, method: %s", url, bflName, shareBy, method)

		req, _ := http.NewRequest(string(c.Request.Method()), url, nil)

		c.Request.Header.VisitAll(func(key, value []byte) {
			req.Header.Set(string(key), string(value))
		})
		req.Header.Set(common.REQUEST_HEADER_OWNER, accessOwner) // external, bflName maybe is owner in host

		body := c.Request.Body()
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
		klog.Errorf("sharePath invalid, type: %s, extend: %s", sharePath.FileType, sharePath.Extend)
		return nil, errors.New("url invalid")
	}

	expired, err := time.Parse(time.RFC3339, sharePath.ExpireTime)
	if err != nil {
		return nil, fmt.Errorf("sharePath expireTime invalid, sharePath.ExpireTime: %s", sharePath.ExpireTime)
	}

	if time.Now().After(expired) {
		return nil, fmt.Errorf("sharePath expireTime, sharePath.ExpireTime: %s", sharePath.ExpireTime)
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

	klog.Infof("share token: %s", common.ToJson(shareToken))

	if shareToken.Token != token {
		return fmt.Errorf("token invalid, shareToken.Token: %s", shareToken.Token)
	}

	expired, err := time.Parse(time.RFC3339, shareToken.ExpireAt)
	if err != nil {
		return fmt.Errorf("shareToken expireAt invalid, shareToken.ExpireAt: %s", shareToken.ExpireAt)
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
func checkPermission(currentUser string, shareBy string, shareType string, permission int32, shareAccess *ShareAccess) bool {
	/**
	 * permission
	 * 0 - no permit
	 * 1 - view, download
	 * 2 - upload only (external only)
	 * 3 - upload, download
	 * 4 - admin
	 */

	if currentUser == shareBy {
		return true
	}

	switch permission {
	case 1:
		return shareAccess.Method == http.MethodGet && !shareAccess.Upload
	case 2: // only upload
		if shareType == common.ShareTypeExternal {
			return shareAccess.Upload || shareAccess.Method == http.MethodGet
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

func accessIntercept(c *app.RequestContext, shared *share.SharePath, shareType string, shareAccess *ShareAccess) bool {
	if shareType == common.ShareTypeExternal {
		if shareAccess.Resource && shareAccess.Method == http.MethodGet {
			klog.Info("share access intercept")
			var f = files.FileInfo{
				FsType:   "share",
				FsExtend: shared.Extend,
				Path:     shared.Path,
				IsDir:    true,
			}
			if shared.Path == "/" {
				f.Name = ""
			} else {
				f.Name = strings.Trim(shared.Path, "/")
			}
			c.Status(200)
			c.Write([]byte(common.ToJson(f)))
			c.Abort()
			return true
		}
	}

	return false
}

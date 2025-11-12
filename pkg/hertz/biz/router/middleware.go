package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"files/pkg/common"
	"files/pkg/global"
	"files/pkg/hertz/biz/dal/database"
	"files/pkg/hertz/biz/handler"
	"files/pkg/hertz/biz/model/api/paste"
	"files/pkg/hertz/biz/model/api/share"
	"files/pkg/models"
	"fmt"
	"io"
	"net/http"
	urlx "net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"k8s.io/klog/v2"
)

type ShareAccess struct {
	Method    string `json:"method"`
	Resource  bool   `json:"resource"`
	Preview   bool   `json:"preview"`
	Raw       bool   `json:"raw"`
	Download  bool   `json:"download"`
	Paste     bool   `json:"paste"`
	Upload    bool   `json:"upload"`
	FromShare bool   `json:"fromShare"`
}

var (
	nonSharePath = []string{
		"/api/nodes",
		"/api/task",
		"/api/accounts",
		"/api/users",
		"/api/share",
		"/api/mounted",
		"/api/mount",
		"/api/unmount",
		"/api/smb_history",
	}
)

var (
	ShareApiResourcesPath  = "/api/resources"
	ShareApiPreviewPath    = "/api/preview"
	ShareApiRawPath        = "/api/raw"
	ShareApiPastePath      = "/api/paste"
	ShareApiUploadPath     = "/upload"
	ShareApiUploadLinkPath = "/upload/upload-link"
	ShareApiUploadedPath   = "/upload/file-uploaded-bytes"
)

func TimingMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		host := string(c.GetHeader("X-Forwarded-Host"))
		path := string(c.Request.RequestURI())

		klog.Infof("%s %s %s starts at %v", string(c.Method()), path, host, start.Format("2006-01-02 15:04:05"))

		defer func() {
			elapsed := time.Since(start)

			klog.Infof("%s %s execution time: %v, code: %d", string(c.Method()), path, elapsed, c.Response.StatusCode())
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
		var cookie = string(c.GetHeader("Cookie"))      // todo for external sharing, this field may be empty unless it contains the sharer's cookie.
		var host = string(c.GetHeader("X-Forwarded-Host"))
		var method = string(c.Request.Method())
		var path = string(c.Request.Path())

		if !checkNonSharedPath(path) {
			c.Next(ctx)
			return
		}

		var paramPath string
		var urlShareType string
		_ = urlShareType
		var shareAccess = &ShareAccess{
			Method: method,
		}

		if strings.HasPrefix(host, "share.") {
			shareAccess.FromShare = true
		}

		var pasteAction, pasteDst string
		var pasteDstParam *models.FileParam

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
		} else if strings.HasPrefix(path, ShareApiPastePath) {
			shareAccess.Paste = true

			var req paste.PasteReq
			err = c.BindAndValidate(&req)
			if err != nil {
				handler.RespError(c, err.Error())
				return
			}

			pasteAction = req.Action
			paramPath = req.Source
			pasteDst = req.Destination

			pasteDstParam, _ = models.CreateFileParam(bflName, pasteDst)

		} else if strings.HasPrefix(path, ShareApiUploadPath) { // ~ upload
			shareAccess.Upload = true
			if strings.HasPrefix(path, ShareApiUploadLinkPath) {
				if method == http.MethodGet {
					paramPath = c.Query("file_path")
					uploadLink = true
				} else {
					if c.Query("share") == "1" {
						paramPath = c.Query("shareby_path")
					} else {
						paramPath = c.PostForm("parent_dir")
					}

					var tPath = strings.TrimPrefix(path, ShareApiUploadLinkPath)
					var tPos = strings.LastIndex(tPath, "/")
					if tPos == 0 {
						handler.RespError(c, "upload-link path invalid")
						return
					}
					uploadLinkId = tPath[tPos+1:]
					if uploadLinkId == "" {
						handler.RespError(c, "upload-link path invalid")
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

		klog.Infof("[share] share param path: %s, bflName: %s, shareAccess: %+v", paramPath, bflName, shareAccess)

		if paramPath == "" {
			c.Next(ctx)
			return
		}

		shareParam, _ := models.CreateFileParam(bflName, paramPath)

		if shareParam == nil || (shareParam.FileType != common.Share && ((pasteDstParam != nil && pasteDstParam.FileType != common.Share) || pasteDstParam == nil)) {
			c.Next(ctx)
			return
		}

		var pasteDstShared *share.SharePath
		var shareExpired int64

		if shareAccess.Paste {

			pasteDstShared, shareExpired, err = checkSharePath(bflName, pasteDstParam.Extend, shareAccess.FromShare)
			if err != nil {
				if shareExpired == 0 {
					handler.RespError(c, common.ErrorMessageWrongShare)
				} else {
					handler.RespErrorExpired(c, common.CodeLinkExpired, common.ErrorMessageLinkExpired, shareExpired)
				}
				return
			}
		}

		shared, expires, err := checkSharePath(bflName, shareParam.Extend, shareAccess.FromShare)
		if err != nil {
			klog.Errorf("[share] check sharePath error: %v", err)
			if expires == 0 {
				handler.RespError(c, common.ErrorMessageWrongShare)
			} else {
				handler.RespErrorExpired(c, common.CodeLinkExpired, common.ErrorMessageLinkExpired, expires)
			}
			return
		}

		urlShareType = strings.ToLower(shared.ShareType)

		klog.Infof("[share] share path: %s", common.ToJson(shared))

		var sharedPath = shared.Path
		var shareType = strings.ToLower(shared.ShareType)
		var shareBy = shared.Owner

		switch shareType {
		case common.ShareTypeInternal:
			if bflName != shared.Owner {
				if err = checkInternal(bflName, shared, shareAccess); err != nil {
					klog.Errorf("[share] check internal error: %v", err)
					handler.RespError(c, "No permission")
					return
				}
			}
		case common.ShareTypeExternal:
			klog.Infof("[share] external share, cookie: %v", len(cookie))
			var token = c.Query("token")
			var expires int64
			var permit bool
			expires, permit, err = checkExternal(bflName, token, shared, shareAccess)
			if err != nil {
				klog.Errorf("[share] check external error: %v, expires: %d", err, expires)
				// handler.RespErrorExpired(c, common.CodeTokenExpired, common.ErrorMessageTokenExpired, expires)
				// return
			} else {
				if !permit {
					// handler.RespError(c, common.ErrorMessagePermissionDenied)
					// return
				}
			}
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
		} else if shareAccess.Paste { // ~ paste
			rewritePrefix = ShareApiPastePath + "/" + global.GlobalNode.GetMasterNode()
		} else if shareAccess.Raw || shareAccess.Download {
			rewritePrefix = ShareApiRawPath
		} else if shareAccess.Upload {
			if uploadBytes {
				rewritePrefix = ShareApiUploadedPath + "/" + global.GlobalNode.GetMasterNode()
			} else {
				rewritePrefix = ShareApiUploadLinkPath + "/" + global.GlobalNode.GetMasterNode()
			}
		} else {
			handler.RespError(c, "No permission")
			return
		}

		if !strings.HasSuffix(rewritePrefix, "/") {
			rewritePrefix += "/"
		}

		var url string
		var accessOwner string
		if shareAccess.Upload { // upload
			if shareType == common.ShareTypeExternal {
				accessOwner = shareBy
			} else {
				accessOwner = bflName
			}

			if uploadLink {
				url = fmt.Sprintf("%s%s?file_path=/%s&from=%s&share=1&sharetype=%s", redirect, rewritePrefix, pathRewrite, c.Query("from"), shareType)
			} else if uploadBytes {
				url = fmt.Sprintf("%s%s?parent_dir=/%s&file_name=%s&share=1&sharetype=%s", redirect, rewritePrefix, pathRewrite, uploadFileName, shareType)
			} else if uploadChunk {
				url = fmt.Sprintf("%s%s%s?&ret-json=%s&share=1&sharetype=%s&shareby=%s&shareby_path=/%s", redirect, rewritePrefix, uploadLinkId, c.Query("ret-json"), shareType, shareBy, pathRewrite)
			}
		} else if shareAccess.Paste {
			accessOwner = bflName
			url += fmt.Sprintf("%s%s?share=1&sharetype=%s", redirect, rewritePrefix, shareType)
		} else {
			accessOwner = shareBy

			url = redirect + rewritePrefix + pathRewrite
			if query != "" {
				url += fmt.Sprintf("?%s&share=1&sharetype=%s", query, shareType)
			} else {
				url += fmt.Sprintf("?share=1&sharetype=%s", shareType)
			}
			if shareAccess.Resource && method == http.MethodGet {
				url += fmt.Sprintf("&sharepermission=%d&shareid=%s&sharepath=%s", shared.Permission, shareParam.Extend, urlx.PathEscape(shareParam.Path))
			}
		}

		//
		klog.Infof("[share] share rewrite url: %s, access: %s, shareby: %s, method: %s", url, bflName, shareBy, method)

		var br io.Reader

		if shareAccess.Paste { // ~ paste
			var req = &paste.PasteReq{
				Action:      pasteAction,
				Source:      fmt.Sprintf("/%s", pathRewrite),
				Destination: fmt.Sprintf("/%s/%s/%s/%s", pasteDstShared.FileType, pasteDstShared.Extend, strings.Trim(pasteDstShared.Path, "/"), strings.TrimLeft(pasteDstParam.Path, "/")),
			}

			bodyBytes, err := json.Marshal(req)
			if err != nil {
				handler.RespError(c, err.Error())
				return
			}
			br = bytes.NewBuffer(bodyBytes)
		}

		var req, _ = http.NewRequest(string(c.Request.Method()), url, br)

		c.Request.Header.VisitAll(func(key, value []byte) {
			req.Header.Set(string(key), string(value))
		})
		if shareAccess.FromShare {
			req.Header.Set(common.REQUEST_HEADER_OWNER, bflName) // external, bflName maybe is owner in host
		} else {
			req.Header.Set(common.REQUEST_HEADER_OWNER, accessOwner) // external, bflName maybe is owner in host
		}

		if !shareAccess.Paste {
			body := c.Request.Body()
			req.Body = io.NopCloser(bytes.NewReader(body))
			req.ContentLength = int64(len(body))
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			handler.RespError(c, fmt.Sprintf("proxy error: %v", err))
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
	}
}

func checkSharePath(currentUser string, shareId string, fromShare bool) (*share.SharePath, int64, error) {
	sharePath, err := database.QueryShareById(shareId)
	if err != nil {
		klog.Errorf("postgres.QueryShareById error: %v", err)
		return nil, 0, errors.New(common.ErrorMessageWrongShare)
	}

	if sharePath == nil {
		klog.Errorf("sharePath not found, shareId: %s", shareId)
		return nil, 0, errors.New(common.ErrorMessageWrongShare)
	}

	if !((sharePath.FileType == common.Drive && sharePath.Extend == common.Home) || sharePath.FileType == common.Sync) {
		klog.Errorf("sharePath invalid, type: %s, extend: %s", sharePath.FileType, sharePath.Extend)
		return nil, 0, errors.New(common.ErrorMessageWrongShare)
	}

	if !fromShare && currentUser == sharePath.Owner {
		return sharePath, 0, nil
	}

	expired, _ := time.Parse(common.DefaultPGTimeFormat, sharePath.ExpireTime)

	if time.Now().After(expired) {
		klog.Errorf("sharePath expired, expireTime: %s", sharePath.ExpireTime)
		return nil, expired.Unix(), errors.New(common.ErrorMessageLinkExpired)
	}

	return sharePath, 0, nil
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

func checkExternal(currentUser string, token string, sharePaths *share.SharePath, shareAccess *ShareAccess) (int64, bool, error) {
	if !shareAccess.FromShare && currentUser == sharePaths.Owner {
		return 0, true, nil
	}
	var defaultExpired = time.Now().Unix()
	token = strings.TrimSpace(token)
	if token == "" {
		return defaultExpired, false, errors.New("token is nil")
	}

	shareToken, err := database.QueryShareExternalById(sharePaths.ID, token)
	if err != nil {
		return defaultExpired, false, fmt.Errorf("postgres.QueryShareExternalById error: %v", err)
	}

	if shareToken == nil {
		return defaultExpired, false, errors.New("shareToken not found")
	}

	klog.Infof("share token: %s", common.ToJson(shareToken))

	expired, _ := time.Parse(common.DefaultPGTimeFormat, shareToken.ExpireAt)
	if time.Now().After(expired) {
		klog.Errorf("[share] shareToken expired, expireAt: %s", shareToken.ExpireAt)
		return expired.Unix(), false, fmt.Errorf("shareToken expired, shareToken.ExpireAt: %s", shareToken.ExpireAt)
	}

	permit := checkPermission(currentUser, sharePaths.Owner, sharePaths.ShareType, sharePaths.Permission, shareAccess)
	return 0, permit, nil
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

	if shareType == common.ShareTypeInternal && currentUser == shareBy {
		return true
	}

	switch permission {
	case 1:
		return shareAccess.Method == http.MethodGet && !shareAccess.Upload
	case 2: // only upload
		if shareType == common.ShareTypeExternal {
			return shareAccess.Upload || (shareAccess.Method == http.MethodGet && shareAccess.Upload)
		}
		return false
	case 3:
		return ((shareAccess.Resource || shareAccess.Preview || shareAccess.Raw) && shareAccess.Method == http.MethodGet) || shareAccess.Upload || shareAccess.Download
	case 4:
		return true
	default:
		return false
	}
}

func checkNonSharedPath(path string) bool {
	var isSharedPath = true
	for _, sp := range nonSharePath {
		if strings.Contains(path, sp) {
			isSharedPath = false
			break
		}
	}

	return isSharedPath
}

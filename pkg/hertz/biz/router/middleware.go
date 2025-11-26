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
	"files/pkg/hertz/biz/model/upload"
	"files/pkg/models"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	urlx "net/url"
	"path/filepath"
	"strconv"
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
		"/api/search",
		"/videos/",
	}
	syncUploadChunks  = "/seafhttp/"
	posixUploadChunks = "/upload/upload-link/"
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

func Cors() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		c.Response.Header.Set("Access-Control-Allow-Origin", string(c.Request.Header.Get("Origin")))
		c.Response.Header.Set("Access-Control-Allow-Credentials", "true")
		c.Response.Header.Set("Access-Control-Allow-Headers", "access-control-allow-headers,access-control-allow-methods,access-control-allow-origin,content-type,x-auth,x-unauth-error,x-authorization")
		c.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Response.Header.Set("Access-Control-Max-Age", "600")

		c.Next(ctx)
	}
}

func Options() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		if bytes.Equal(ctx.Method(), []byte("OPTIONS")) {
			origin := string(ctx.Request.Header.Peek("Origin"))
			if origin != "" {
				ctx.Header("Access-Control-Allow-Origin", origin)
				ctx.Header("Vary", "Origin")
			}
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			ctx.Header("Access-Control-Max-Age", "86400")
			ctx.Status(204)
			return
		}
		ctx.Next(c)
	}
}

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

		// if Paste, is's Source
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
		// if Paste, it's Destination
		var pasteDstParam *models.FileParam

		var uploadLink, uploadBytes, uploadChunk bool
		var uploadFileName string

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
		} else if strings.HasPrefix(path, ShareApiPastePath) { // paste
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
					uploadChunk = true
				}
			} else if strings.HasPrefix(path, ShareApiUploadedPath) {
				paramPath = c.Query("parent_dir")
				uploadFileName = c.Query("file_name")
				uploadBytes = true
			}
		}

		klog.Infof("[share] share param path: %s, bflName: %s, shareAccess: %+v", paramPath, bflName, shareAccess)

		if paramPath == "" || uploadChunk {
			c.Next(ctx)
			return
		}

		// if Paste, it's Source
		var shareParam *models.FileParam
		shareParam, _ = models.CreateFileParam(bflName, paramPath)

		if shareParam == nil || (shareParam.FileType != common.Share && ((pasteDstParam != nil && pasteDstParam.FileType != common.Share) || pasteDstParam == nil)) {
			c.Next(ctx)
			return
		}

		if shareAccess.Paste {
			proxySharePaste(c, bflName, pasteAction, shareParam, pasteDstParam)
			return
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
		var shareMember *share.ShareMember

		switch shareType {
		case common.ShareTypeInternal:
			if bflName != shared.Owner {
				shareMember, err = checkInternal(bflName, shared, shareAccess)
				if err != nil {
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
				handler.RespErrorExpired(c, common.CodeTokenExpired, common.ErrorMessageTokenExpired, expires)
				return
			} else {
				klog.Infof("[share] check external, permit: %v, expires: %d, shareOwner: %s, shareType: %s, sharePermit: %d", permit, expires, shared.Owner, shared.ShareType, shared.Permission)
				if !permit {
					if shared.Permission == 2 {
						handler.RespSuccess(c, nil)
						return
					}
					handler.RespError(c, common.ErrorMessagePermissionDenied)
					return
				}
			}
		}

		var masterNodeName = global.GlobalNode.GetMasterNode()
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
				rewritePrefix = ShareApiUploadedPath + "/" + masterNodeName
			} else {
				rewritePrefix = ShareApiUploadLinkPath + "/" + masterNodeName
			}
		} else {
			handler.RespError(c, "No permission")
			return
		}

		if strings.HasSuffix(rewritePrefix, "/") {
			rewritePrefix = strings.TrimSuffix(rewritePrefix, "/")
		}

		if !strings.HasPrefix(pathRewrite, "/") {
			pathRewrite = "/" + pathRewrite
		}

		pathRewrite = common.EscapeURLWithSpace(pathRewrite)

		var permission int32
		if shared.ShareType == common.ShareTypeInternal {
			if shareMember != nil {
				permission = shareMember.Permission
			} else {
				permission = shared.Permission
			}
		} else {
			permission = shared.Permission
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
				url = fmt.Sprintf("%s%s?file_path=%s&from=%s&share=1&sharetype=%s&shareby=%s", redirect, rewritePrefix, pathRewrite, c.Query("from"), shareType, shareBy)
			} else if uploadBytes {
				url = fmt.Sprintf("%s%s?parent_dir=%s&file_name=%s&share=1&sharetype=%s&shareby=%s", redirect, rewritePrefix, pathRewrite, uploadFileName, shareType, shareBy)
			}
		} else {
			accessOwner = shareBy

			url = redirect + rewritePrefix + pathRewrite
			if query != "" {
				url += fmt.Sprintf("?%s&share=1&sharetype=%s", query, shareType)
			} else {
				url += fmt.Sprintf("?share=1&sharetype=%s", shareType)
			}
			if shareAccess.Resource && method == http.MethodGet {
				url += fmt.Sprintf("&sharepermission=%d&shareid=%s&sharepath=%s", permission, shareParam.Extend, urlx.PathEscape(shareParam.Path))
			}
		}

		//
		klog.Infof("[share] share rewrite url: %s, access: %s, shareby: %s, method: %s", url, bflName, shareBy, method)

		var br io.Reader
		var req, _ = http.NewRequest(string(c.Request.Method()), url, br)

		c.Request.Header.VisitAll(func(key, value []byte) {
			req.Header.Set(string(key), string(value))
		})
		if shareAccess.FromShare {
			req.Header.Set(common.REQUEST_HEADER_OWNER, bflName) // external, bflName maybe is owner in host
		} else {
			req.Header.Set(common.REQUEST_HEADER_OWNER, accessOwner) // external, bflName maybe is owner in host
		}

		body := c.Request.Body()
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))

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

	if !fromShare && currentUser == sharePath.Owner {
		return sharePath, 0, nil
	}

	expired, _ := time.Parse(time.RFC3339Nano, sharePath.ExpireTime)

	if time.Now().After(expired) {
		klog.Errorf("sharePath expired, expireTime: %s", sharePath.ExpireTime)
		return nil, expired.Unix(), errors.New(common.ErrorMessageLinkExpired)
	}

	return sharePath, 0, nil
}

func checkInternal(currentOwner string, sharePaths *share.SharePath, shareAccess *ShareAccess) (*share.ShareMember, error) {
	shareMember, err := database.QueryShareMemberById(currentOwner, sharePaths.ID)
	if err != nil {
		return nil, fmt.Errorf("postgres.QueryShareMemberById error: %v", err)
	}

	if shareMember.ShareMember == "" {
		return nil, errors.New("shareMember not found")
	}

	// permission is shareMember.Permission
	if permit := checkPermission(currentOwner, sharePaths.Owner, sharePaths.ShareType, shareMember.Permission, shareAccess); !permit {
		return nil, errors.New("authorization check failed")
	}
	return shareMember, nil
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

	expired, _ := time.Parse(time.RFC3339Nano, shareToken.ExpireAt)
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
		return true
		// return ((shareAccess.Resource || shareAccess.Preview || shareAccess.Raw) && shareAccess.Method == http.MethodGet) || shareAccess.Upload || shareAccess.Download
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

func proxySharePaste(c *app.RequestContext, owner string, action string, src, dst *models.FileParam) {
	var isSrcShare, isDstShare = src.FileType == common.Share, dst.FileType == common.Share

	klog.Infof("[share] Paste, owern: %s, src: %s, dst: %s", owner, common.ParseString(src), common.ParseString(dst))

	var srcDriveParam, dstDriveParam *models.FileParam
	var srcShareType, dstShareType string

	if isSrcShare {
		shared, err := database.GetSharePath(src.Extend)
		if err != nil {
			handler.RespError(c, fmt.Sprintf("Get Share Source Error: %v", err))
			return
		}

		if shared == nil {
			handler.RespError(c, common.ErrorMessagePasteWrongSourceShare)
			return
		}

		// check expire
		if checkExpired(shared.ExpireTime) {
			handler.RespError(c, common.ErrorMessagePasteSourceExpired)
			return
		}

		// check member
		member, err := database.GetShareMember(shared.ID, owner)
		if err != nil {
			handler.RespError(c, fmt.Sprintf("Get Share Source Member Error: %v", err))
			return
		}
		if member == nil {
			handler.RespError(c, common.ErrorMessagePasteWrongSourceShare)
			return
		}

		// check permission
		if member.Permission < 1 {
			handler.RespError(c, common.ErrorMessagePermissionDenied)
			return
		}

		//
		srcShareType = shared.ShareType
		srcDriveParam = &models.FileParam{
			Owner:    shared.Owner,
			FileType: shared.FileType,
			Extend:   shared.Extend,
			Path:     shared.Path + strings.TrimPrefix(src.Path, "/"),
		}
	} else {
		srcDriveParam = &models.FileParam{
			Owner:    owner,
			FileType: src.FileType,
			Extend:   src.Extend,
			Path:     src.Path,
		}
	}

	if isDstShare {
		shared, err := database.GetSharePath(dst.Extend)
		if err != nil {
			handler.RespError(c, fmt.Sprintf("Get Share Destination Error: %v", err))
			return
		}

		if shared == nil {
			handler.RespError(c, common.ErrorMessagePasteWrongDestinationShare)
			return
		}

		// check expire
		if checkExpired(shared.ExpireTime) {
			handler.RespError(c, common.ErrorMessagePasteDestinationExpired)
			return
		}

		// check member
		if owner != shared.Owner {
			member, err := database.GetShareMember(shared.ID, owner)
			if err != nil {
				handler.RespError(c, fmt.Sprintf("Get Share Destination Member Error: %v", err))
				return
			}

			if member == nil {
				handler.RespError(c, common.ErrorMessagePasteWrongDestinationShare)
				return
			}

			// check permission, view, edit, admin
			if owner != shared.Owner && member.Permission < 2 {
				handler.RespError(c, common.ErrorMessagePermissionDenied)
				return
			}
		}

		//
		dstShareType = shared.ShareType
		dstDriveParam = &models.FileParam{
			Owner:    shared.Owner,
			FileType: shared.FileType,
			Extend:   shared.Extend,
			Path:     shared.Path + strings.TrimPrefix(dst.Path, "/"),
		}
	} else {
		dstDriveParam = &models.FileParam{
			Owner:    owner,
			FileType: dst.FileType,
			Extend:   dst.Extend,
			Path:     dst.Path,
		}
	}

	var param = &paste.PasteReq{
		Action:       action,
		Source:       fmt.Sprintf("/%s/%s/%s", srcDriveParam.FileType, srcDriveParam.Extend, strings.TrimPrefix(srcDriveParam.Path, "/")),
		Destination:  fmt.Sprintf("/%s/%s/%s", dstDriveParam.FileType, dstDriveParam.Extend, strings.TrimPrefix(dstDriveParam.Path, "/")),
		Share:        1,
		SrcShareType: srcShareType,
		DstShareType: dstShareType,
		SrcOwner:     srcDriveParam.Owner,
		DstOwner:     dstDriveParam.Owner,
	}

	var masterNodeName = global.GlobalNode.GetMasterNode()
	var query = string(c.Request.QueryString())
	var url = fmt.Sprintf("http://%s/%s/%s?%s", common.SERVER_HOST, strings.TrimPrefix(ShareApiPastePath, "/"), masterNodeName, query)

	var br io.Reader
	bodyBytes, err := json.Marshal(param)
	if err != nil {
		handler.RespError(c, err.Error())
		return
	}
	br = bytes.NewBuffer(bodyBytes)

	var req, _ = http.NewRequest(string(c.Request.Method()), url, br)

	c.Request.Header.VisitAll(func(key, value []byte) {
		req.Header.Set(string(key), string(value))
	})
	req.Header.Set(common.REQUEST_HEADER_OWNER, owner)

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

func checkExpired(expireAt string) bool {
	expired, _ := time.Parse(time.RFC3339Nano, expireAt)
	if time.Now().After(expired) {
		return true
	}
	return false
}

func ShareUpload() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		var err error
		var reqMethod = string(ctx.Request.Method())
		var path = string(ctx.Request.Path())

		if !((strings.HasPrefix(path, syncUploadChunks) || strings.HasPrefix(path, posixUploadChunks)) && reqMethod == http.MethodPost) {
			ctx.Next(c)
			return
		}

		var uploadReq upload.UploadChunksReq
		if err = ctx.BindAndValidate(&uploadReq); err != nil {
			klog.Errorf("Sync uploadChunks, bind and validate error: %v", err)
			handler.RespBadRequest(ctx, err.Error())
			return
		}

		var share = uploadReq.Share
		if share != "1" {
			ctx.Next(c)
			return
		}

		owner := string(ctx.GetHeader(common.REQUEST_HEADER_OWNER))
		if owner == "" {
			handler.RespBadRequest(ctx, "user not found")
			return
		}

		fp, _ := models.CreateFileParam(owner, uploadReq.ParentDir)

		shared, err := database.GetSharePath(fp.Extend)
		if err != nil {
			klog.Errorf("Sync uploadChunks, share get error: %v", err)
			handler.RespBadRequest(ctx, err.Error())
			return
		}
		if shared == nil {
			handler.RespBadRequest(ctx, common.ErrorMessageWrongShare)
			return
		}

		mf, err := ctx.MultipartForm()
		if err != nil {
			klog.Errorf("Sync uploadChunks, parse multipart error: %v", err)
			handler.RespBadRequest(ctx, err.Error())
			return
		}
		defer mf.RemoveAll()

		var hasPathname, hasRepoId, hasDriveType bool
		var hasShareBy bool
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)

		for name := range mf.Value {
			switch name {
			case "pathname":
				hasPathname = true
			case "repoId":
				hasRepoId = true
			case "driveType":
				hasDriveType = true
			case "shareby":
				hasShareBy = true
			}
		}

		for name, vals := range mf.Value {
			switch name {
			case "parent_dir", "pathname":
				if shared.FileType == common.Sync {
					err = createPart(name, []string{shared.Path + strings.TrimPrefix(fp.Path, "/")}, mw)
				} else {
					err = createPart(name, []string{fmt.Sprintf("/%s/%s/%s", shared.FileType, shared.Extend, strings.TrimPrefix(shared.Path, "/")+strings.TrimPrefix(fp.Path, "/"))}, mw)
				}
			case "repoId":
				if shared.FileType == common.Sync {
					err = createPart(name, []string{shared.Extend}, mw)
				}
			case "driveType":
				err = createPart(name, []string{shared.FileType}, mw)
			default:
				err = createPart(name, vals, mw)
			}

			if err != nil {
				break
			}
		}

		if !hasPathname {
			if shared.FileType == common.Sync {
				err = createPart("pathname", []string{shared.Path + strings.TrimPrefix(fp.Path, "/")}, mw)
			} else {
				err = createPart("pathname", []string{fmt.Sprintf("/%s/%s/%s", shared.FileType, shared.Extend, strings.TrimPrefix(shared.Path, "/")+strings.TrimPrefix(fp.Path, "/"))}, mw)
			}
		}
		if !hasRepoId && shared.FileType == common.Sync {
			err = createPart("repoId", []string{shared.Extend}, mw)
		}
		if !hasDriveType {
			err = createPart("driveType", []string{shared.FileType}, mw)
		}
		if !hasShareBy && shared.FileType == common.Drive {
			err = createPart("shareby", []string{shared.Owner}, mw)
		}

		if err != nil {
			klog.Errorf("Sync uploadChunks, create MultipartForm error: %v", err)
			handler.RespBadRequest(ctx, err.Error())
			return
		}

		for field, fhs := range mf.File {
			for _, fh := range fhs {
				src, err := fh.Open()
				if err != nil {
					ctx.String(http.StatusInternalServerError, "open file err: %v", err)
					_ = mw.Close()
					return
				}
				part, err := mw.CreateFormFile(field, fh.Filename)
				if err != nil {
					src.Close()
					ctx.String(http.StatusInternalServerError, "create form file err: %v", err)
					_ = mw.Close()
					return
				}
				if _, err := io.Copy(part, src); err != nil {
					src.Close()
					ctx.String(http.StatusInternalServerError, "copy file err: %v", err)
					_ = mw.Close()
					return
				}
				src.Close()
			}
		}

		if err := mw.Close(); err != nil {
			ctx.String(http.StatusInternalServerError, "close mw err: %v", err)
			return
		}

		newCT := mw.FormDataContentType()
		ctx.Request.SetBody(buf.Bytes())
		ctx.Request.Header.Set("Content-Type", newCT)
		ctx.Request.Header.Set("Content-Length", strconv.Itoa(buf.Len()))

		ctx.Next(c)
	}

}

func createPart(name string, vals []string, mw *multipart.Writer) error {
	for _, v := range vals {
		hdr := make(textproto.MIMEHeader)
		hdr.Set("Content-Disposition", `form-data; name="`+name+`"; attr="modified"`)

		part, err := mw.CreatePart(hdr)
		if err != nil {
			_ = mw.Close()
			return fmt.Errorf("create part %s error: %v", name, err)
		}

		if _, err := io.WriteString(part, v); err != nil {
			_ = mw.Close()
			return fmt.Errorf("write part %s %v error: %v", name, v, err)
		}
	}

	return nil
}

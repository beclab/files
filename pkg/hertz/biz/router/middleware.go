package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"files/pkg/access"
	"files/pkg/common"
	"files/pkg/global"
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
		"/api/mounted_states",
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

// shareProxyClient is used to reverse-proxy share-API requests. Bodies on
// either side may be GB-scale (file uploads/downloads), so Client.Timeout is
// intentionally NOT set; that would clip legitimate long transfers.
// ResponseHeaderTimeout instead guards against an origin that never starts
// sending headers, while idle-conn and TLS-handshake timeouts cap dead
// connection reuse and slow handshakes.
var shareProxyClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConnsPerHost:   64,
		DisableCompression:    true,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second,
	},
}

// Cors sets per-response CORS headers. It echoes Origin only when
// common.AllowedOrigin authorizes it (same effective host, or a host
// listed in $CORS_ALLOWED_ORIGINS). Reflecting an arbitrary Origin
// alongside Allow-Credentials: true - the previous behavior - lets
// any cross-site page issue credentialed XHR against this service,
// so the header is now omitted for unrelated origins and the browser
// blocks the response from JavaScript.
func Cors() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		origin := string(c.Request.Header.Get("Origin"))
		forwardedHost := string(c.GetHeader("X-Forwarded-Host"))
		host := string(c.Request.Host())
		if allowed := common.AllowedOrigin(origin, forwardedHost, host); allowed != "" {
			c.Response.Header.Set("Access-Control-Allow-Origin", allowed)
			c.Response.Header.Set("Access-Control-Allow-Credentials", "true")
			c.Response.Header.Add("Vary", "Origin")
		}
		c.Response.Header.Set("Access-Control-Allow-Headers", "access-control-allow-headers,access-control-allow-methods,access-control-allow-origin,content-type,x-auth,x-unauth-error,x-authorization,x-archive-password")
		c.Response.Header.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Response.Header.Set("Access-Control-Max-Age", "600")

		c.Next(ctx)
	}
}

func Options() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		if bytes.Equal(ctx.Method(), []byte("OPTIONS")) {
			origin := string(ctx.Request.Header.Peek("Origin"))
			forwardedHost := string(ctx.GetHeader("X-Forwarded-Host"))
			host := string(ctx.Request.Host())
			if allowed := common.AllowedOrigin(origin, forwardedHost, host); allowed != "" {
				ctx.Header("Access-Control-Allow-Origin", allowed)
				ctx.Header("Access-Control-Allow-Credentials", "true")
				ctx.Header("Vary", "Origin")
			}
			ctx.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
			ctx.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, X-Archive-Password")
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
		bflName := string(c.GetHeader(common.REQUEST_HEADER_OWNER))
		newCookie := string(c.GetHeader("Cookie"))

		if bflName != "" {
			oldCookie, _ := common.BflCookieGet(bflName)
			if newCookie != oldCookie {
				common.BflCookieSet(bflName, newCookie)
			}
		}

		klog.V(4).Infof("BflCookieCache updated for user=%s", bflName)
		c.Next(ctx)
	}
}

// + share
func ShareMiddleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		var err error
		var bflName = string(c.GetHeader(common.REQUEST_HEADER_OWNER)) // todo if sharing externally, you may use the name from the host here.
		var host = string(c.GetHeader("X-Forwarded-Host"))
		var method = string(c.Request.Method())
		var path = string(c.Request.Path())

		if !checkNonSharedPath(path) {
			c.Next(ctx)
			return
		}

		// if Paste, is's Source
		var paramPath string
		var shareAccess = &access.ShareAccess{
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

		} else if strings.HasPrefix(path, ShareApiUploadPath) { // ! upload
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
		shareParam, err = models.CreateFileParam(bflName, paramPath)
		if err != nil {
			klog.Errorf("[share] CreateFileParam error: %v, path: %s", err, paramPath)
		}

		if shareParam == nil || (shareParam.FileType != common.Share && ((pasteDstParam != nil && pasteDstParam.FileType != common.Share) || pasteDstParam == nil)) {
			c.Next(ctx)
			return
		}

		if shareAccess.Paste {
			proxySharePaste(ctx, c, bflName, pasteAction, shareParam, pasteDstParam)
			return
		}

		if shareParam.FileType == common.Share {
			shareParam.Extend = common.TrimShareId(shareParam.Extend, global.GlobalNode.CheckNodeExists)
		}

		shared, expires, err := access.ShareResolvePath(bflName, shareParam.Extend, shareAccess.FromShare)
		if err != nil {
			klog.Errorf("[share] check sharePath error: %v", err)
			if expires == 0 {
				handler.RespError(c, common.ErrorMessageWrongShare)
			} else {
				handler.RespErrorExpired(c, common.CodeLinkExpired, common.ErrorMessageLinkExpired, expires)
			}
			return
		}

		klog.Infof("[share] share path: %s", common.ToJson(shared))

		var shareNode string
		var sharedPath = shared.Path
		var shareType = strings.ToLower(shared.ShareType)
		var shareBy = shared.Owner
		var shareMember *share.ShareMember
		var shareFileType = shared.FileType

		if shared.FileType == common.External || shared.FileType == common.Cache {
			shareNode = urlx.PathEscape(shared.Extend)
		} else {
			shareNode = global.GlobalNode.GetMasterNode()
		}

		shareMember, expires, err = access.ShareAuthorize(bflName, c.Query("token"), shared, shareAccess)
		if err != nil {
			if expires > 0 {
				klog.Errorf("[share] authorize error: %v, expires: %d", err, expires)
				handler.RespErrorExpired(c, common.CodeTokenExpired, common.ErrorMessageTokenExpired, expires)
			} else {
				klog.Errorf("[share] authorize denied: %v", err)
				handler.RespForbidden(c, common.ErrorMessagePermissionDenied)
			}
			return
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
			handler.RespError(c, common.ErrorMessagePermissionDenied)
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
			if shareType == common.ShareTypeExternal || (shareType == common.ShareTypeInternal && shared.FileType == common.Sync) {
				accessOwner = shareBy
			} else {
				accessOwner = bflName
			}

			if uploadLink {
				url = fmt.Sprintf("%s%s?file_path=%s&from=%s&share=1&sharetype=%s&shareby=%s", redirect, rewritePrefix, pathRewrite, common.EscapeURLWithSpace(c.Query("from")), shareType, shareBy)
				if c.Query("total_size") != "" {
					url += "&total_size=" + c.Query("total_size")
				}
			} else if uploadBytes {
				url = fmt.Sprintf("%s%s?parent_dir=%s&file_name=%s&share=1&sharetype=%s&shareby=%s", redirect, rewritePrefix, pathRewrite, common.EscapeURLWithSpace(uploadFileName), shareType, shareBy)
			}
		} else {
			accessOwner = shareBy

			url = redirect + rewritePrefix + pathRewrite
			if query != "" {
				url += fmt.Sprintf("?%s&share=1&sharetype=%s&sharenode=%s&sharefiletype=%s", query, shareType, shareNode, shareFileType)
			} else {
				url += fmt.Sprintf("?share=1&sharetype=%s&sharenode=%s&sharefiletype=%s", shareType, shareNode, shareFileType)
			}
			if shareAccess.Resource && method == http.MethodGet {
				url += fmt.Sprintf("&sharepermission=%d&shareid=%s&sharepath=%s&sharenode=%s&sharefiletype=%s", permission, shareParam.Extend, urlx.PathEscape(shareParam.Path), shareNode, shareFileType)
			}
		}

		//
		klog.Infof("[share] share rewrite url: %s, access: %s, shareby: %s, method: %s", url, bflName, shareBy, method)

		var br io.Reader
		// Tie the proxied request to the inbound request's ctx so a
		// disconnect / cancellation propagates downstream and the
		// shareProxyClient.Do call doesn't hang past the original
		// caller's lifetime.
		req, err := http.NewRequestWithContext(ctx, string(c.Request.Method()), url, br)
		if err != nil {
			klog.Errorf("[share] build proxy request error: %v, url: %s", err, url)
			handler.RespError(c, fmt.Sprintf("build proxy request error: %v", err))
			return
		}

		c.Request.Header.VisitAll(func(key, value []byte) {
			req.Header.Set(string(key), string(value))
		})
		if shareAccess.FromShare {
			req.Header.Set(common.REQUEST_HEADER_OWNER, bflName) // external, bflName maybe is owner in host
		} else {
			req.Header.Set(common.REQUEST_HEADER_OWNER, accessOwner) // external, bflName maybe is owner in host
		}
		// Mark this as a server-side share-resolved forward so the
		// downstream handler's Gate trusts the share=1 query. See
		// pkg/common/internal_auth.go.
		req.Header.Set(common.HeaderInternalShareToken, common.InternalShareToken())

		body := c.Request.Body()
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))

		resp, err := shareProxyClient.Do(req)
		if err != nil {
			handler.RespError(c, fmt.Sprintf("proxy error: %v", err))
			return
		}

		for k, vv := range resp.Header {
			for _, v := range vv {
				c.Header(k, v)
			}
		}
		c.Status(resp.StatusCode)

		if shareAccess.Raw || shareAccess.Download || shareAccess.Preview {
			contentLength := int(resp.ContentLength)
			if contentLength < 0 {
				contentLength = -1
			}
			c.SetBodyStream(resp.Body, contentLength)
		} else {
			bodyRes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			// c.Write writes to the Hertz response buffer; failures
			// here are unactionable (we can't alter the response any
			// more), drop explicitly to satisfy errcheck.
			_, _ = c.Write(bodyRes)
		}
		c.Abort()
	}
}

// checkNonSharedPath reports whether path should be processed by the
// share middleware. It returns false (i.e. skip share middleware) when
// path is, or sits under, one of the well-known non-share routes in
// nonSharePath.
//
// Matching is path-segment based: an entry "/api/share" matches
// "/api/share" and "/api/share/<anything>" but NOT "/api/sharefoo" or
// "/api/share-thing". An entry that already ends with "/" (e.g.
// "/videos/") behaves as a literal prefix. The previous implementation
// used strings.Contains, which let attacker-controlled prefixes such as
// "/api/sharefoo" silently bypass the share authorization middleware.
func checkNonSharedPath(path string) bool {
	for _, sp := range nonSharePath {
		if strings.HasSuffix(sp, "/") {
			if strings.HasPrefix(path, sp) {
				return false
			}
			continue
		}
		if path == sp || strings.HasPrefix(path, sp+"/") {
			return false
		}
	}
	return true
}

func proxySharePaste(ctx context.Context, c *app.RequestContext, owner string, action string, src, dst *models.FileParam) {
	var isSrcShare, isDstShare = src.FileType == common.Share, dst.FileType == common.Share

	klog.Infof("[share] Paste, owner: %s, src: %s, dst: %s", owner, common.ParseString(src), common.ParseString(dst))

	if src.FileType == common.Share {
		src.Extend = common.TrimShareId(src.Extend, global.GlobalNode.CheckNodeExists)
	}

	if dst.FileType == common.Share {
		dst.Extend = common.TrimShareId(dst.Extend, global.GlobalNode.CheckNodeExists)
	}

	var srcDriveParam, dstDriveParam *models.FileParam
	var srcShareType, dstShareType string

	if isSrcShare {
		shared, err := access.ShareCheckPaste(owner, src.Extend, false)
		if err != nil {
			switch {
			case errors.Is(err, access.ErrShareNotFound):
				handler.RespError(c, common.ErrorMessagePasteWrongSourceShare)
			case errors.Is(err, access.ErrShareExpired):
				handler.RespError(c, common.ErrorMessagePasteSourceExpired)
			case errors.Is(err, access.ErrShareDenied):
				handler.RespForbidden(c, common.ErrorMessagePermissionDenied)
			default:
				handler.RespError(c, fmt.Sprintf("Get Share Source Error: %v", err))
			}
			return
		}

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
		shared, err := access.ShareCheckPaste(owner, dst.Extend, true)
		if err != nil {
			switch {
			case errors.Is(err, access.ErrShareNotFound):
				handler.RespError(c, common.ErrorMessagePasteWrongDestinationShare)
			case errors.Is(err, access.ErrShareExpired):
				handler.RespError(c, common.ErrorMessagePasteDestinationExpired)
			case errors.Is(err, access.ErrShareDenied):
				handler.RespForbidden(c, common.ErrorMessagePermissionDenied)
			default:
				handler.RespError(c, fmt.Sprintf("Get Share Destination Error: %v", err))
			}
			return
		}

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
		SrcSharePath: fmt.Sprintf("/%s/%s/%s", src.FileType, src.Extend, strings.TrimPrefix(src.Path, "/")),
		DstSharePath: fmt.Sprintf("/%s/%s/%s", dst.FileType, dst.Extend, strings.TrimPrefix(dst.Path, "/")),
	}

	var masterNodeName = global.GlobalNode.GetMasterNode()
	var query = string(c.Request.QueryString())
	var url = fmt.Sprintf("http://%s/%s/%s?%s", common.SERVER_HOST, strings.TrimPrefix(ShareApiPastePath, "/"), masterNodeName, query)

	klog.Infof("[share] share paste rewrite url: %s", url)

	var br io.Reader
	bodyBytes, err := json.Marshal(param)
	if err != nil {
		handler.RespError(c, err.Error())
		return
	}
	br = bytes.NewBuffer(bodyBytes)

	// Tie the proxied request to the inbound ctx so disconnects /
	// cancellations propagate downstream. The previous bare
	// http.NewRequest(...) also discarded its error, leaving req
	// potentially nil before the immediate Header.Set calls
	// nil-deref'd; surface that error explicitly now.
	req, err := http.NewRequestWithContext(ctx, string(c.Request.Method()), url, br)
	if err != nil {
		klog.Errorf("[share] proxy paste build request error: %v, url: %s", err, url)
		handler.RespError(c, fmt.Sprintf("build proxy request error: %v", err))
		return
	}

	c.Request.Header.VisitAll(func(key, value []byte) {
		req.Header.Set(string(key), string(value))
	})
	req.Header.Set(common.REQUEST_HEADER_OWNER, owner)
	// Mark this as a server-side share-resolved forward; the paste
	// handler trusts SrcOwner/DstOwner only when this header matches
	// the process-local secret. See pkg/common/internal_auth.go.
	req.Header.Set(common.HeaderInternalShareToken, common.InternalShareToken())

	resp, err := shareProxyClient.Do(req)
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
	// see notes above: Hertz response buffer; can't surface err.
	_, _ = c.Write(bodyRes)
	c.Abort()
}

func ShareUpload() app.HandlerFunc {
	return func(c context.Context, ctx *app.RequestContext) {
		var err error
		var reqMethod = string(ctx.Request.Method())
		var path = string(ctx.Request.Path())
		var host = string(ctx.GetHeader("X-Forwarded-Host"))

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
		if share != "1" && !strings.HasPrefix(host, "share.") {
			ctx.Next(c)
			return
		}

		owner := string(ctx.GetHeader(common.REQUEST_HEADER_OWNER))
		if owner == "" {
			handler.RespBadRequest(ctx, "user not found")
			return
		}

		fp, err := models.CreateFileParam(owner, uploadReq.ParentDir)
		if err != nil || fp == nil {
			klog.Errorf("Sync uploadChunks, file param error: %v, parentDir: %s", err, uploadReq.ParentDir)
			handler.RespBadRequest(ctx, common.ErrorMessageWrongShare)
			return
		}
		if fp.FileType == common.Share {
			fp.Extend = common.TrimShareId(fp.Extend, global.GlobalNode.CheckNodeExists)
		}

		// Resolve the share with the same expiry enforcement as the read
		// path (ShareMiddleware): GetSharePath alone skips share_paths
		// expiry, so an expired internal share could still receive chunks.
		fromShare := strings.HasPrefix(host, "share.")
		shared, expires, err := access.ShareResolvePath(owner, fp.Extend, fromShare)
		if err != nil {
			klog.Errorf("Sync uploadChunks, share resolve error: %v, expires: %d", err, expires)
			if expires > 0 {
				handler.RespErrorExpired(ctx, common.CodeLinkExpired, common.ErrorMessageLinkExpired, expires)
			} else {
				handler.RespError(ctx, common.ErrorMessageWrongShare)
			}
			return
		}

		// Gate chunk uploads through the same share permission matrix
		// as the non-chunk upload-link path; ShareMiddleware skips chunk
		// POSTs, so without this a read-only member could push chunks.
		uploadAccess := &access.ShareAccess{
			Method:    http.MethodPost,
			Upload:    true,
			FromShare: fromShare,
		}
		if _, exp, err := access.ShareAuthorize(owner, string(ctx.Query("token")), shared, uploadAccess); err != nil {
			klog.Errorf("[share] uploadChunks authorize error: %v, expires: %d", err, exp)
			if exp > 0 {
				handler.RespErrorExpired(ctx, common.CodeTokenExpired, common.ErrorMessageTokenExpired, exp)
			} else {
				handler.RespForbidden(ctx, common.ErrorMessagePermissionDenied)
			}
			return
		}

		mf, err := ctx.MultipartForm()
		if err != nil {
			klog.Errorf("Sync uploadChunks, parse multipart error: %v", err)
			handler.RespBadRequest(ctx, err.Error())
			return
		}
		// Best-effort cleanup of multipart temp files.
		defer func() {
			if e := mf.RemoveAll(); e != nil {
				klog.Warningf("multipart RemoveAll failed: %v", e)
			}
		}()

		var hasPathname, hasRepoId, hasDriveType bool
		var hasShareBy bool
		// Spool the rebuilt multipart body to disk above
		// SpoolDefaultMemLimit. Hertz already caps inbound bodies
		// at 20 MiB, but copying every uploaded file into a single
		// in-memory bytes.Buffer doubled the per-request peak; the
		// spool keeps it bounded regardless of how many uploads are
		// in flight concurrently. Cleanup deletes the temp file once
		// the downstream handler has finished consuming the stream.
		spool := common.NewSpoolWriter(common.SpoolDefaultMemLimit)
		defer func() {
			if e := spool.Cleanup(); e != nil {
				klog.Warningf("share upload spool cleanup: %v", e)
			}
		}()
		mw := multipart.NewWriter(spool)

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
		if err == nil {
			err = createPart("sharebyPath", []string{uploadReq.ParentDir}, mw)
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
		bodyReader, err := spool.Reader()
		if err != nil {
			klog.Errorf("Sync uploadChunks, spool reader error: %v", err)
			handler.RespBadRequest(ctx, err.Error())
			return
		}
		bodySize := spool.Size()
		ctx.Request.SetBodyStream(bodyReader, int(bodySize))
		ctx.Request.Header.Set("Content-Type", newCT)
		ctx.Request.Header.Set("Content-Length", strconv.FormatInt(bodySize, 10))

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

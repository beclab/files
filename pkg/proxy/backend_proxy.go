// Copyright 2023 bytetrade
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"files/pkg/appdata"
	"files/pkg/drives"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	API_RESOURCES_PREFIX           = "/api/resources/AppData"
	API_RAW_PREFIX                 = "/api/raw/AppData"
	API_MD5_PREFIX                 = "/api/md5/AppData"
	API_PREVIEW_THUMB_PREFIX       = "/api/preview/thumb/AppData"
	API_PREVIEW_BIG_PREFIX         = "/api/preview/big/AppData"
	API_PERMISSION_PREFIX          = "/api/permission/AppData"
	API_RESOURCES_CACHE_PREFIX     = "/api/resources/cache"
	API_RAW_CACHE_PREFIX           = "/api/raw/cache"
	API_MD5_CACHE_PREFIX           = "/api/md5/cache"
	API_PREVIEW_THUMB_CACHE_PREFIX = "/api/preview/thumb/cache"
	API_PREVIEW_BIG_CACHE_PREFIX   = "/api/preview/big/cache"
	API_PERMISSION_CACHE_PREFIX    = "/api/permission/cache"
	NODE_HEADER                    = "X-Terminus-Node"
	BFL_HEADER                     = "X-Bfl-User"
	API_PREFIX                     = "/api"
	UPLOADER_PREFIX                = "/upload"
	MEDIA_PREFIX                   = "/videos"
	API_PASTE_PREFIX               = "/api/paste"
	API_PASTE_CACHE_PREFIX         = "/api/paste/cache"
	API_CACHE_PREFIX               = "/api/cache"
	API_BATCH_DELETE_PREFIX        = "/api/batch_delete"
)

var REWRITE_FOCUSED_PREFIXES = []string{
	"/api/resources/",
	"/api/raw/",
	"/api/preview/thumb/",
	"/api/preview/big/",
	"/api/paste/",
	"/api/md5/",
	"/api/permission/",
}

var PVCs *PVCCache = nil

type GatewayHandler func(c echo.Context) (next bool, err error)

type BackendProxyBuilder struct {
	Verbose    bool
	Addr       string
	MainCtx    context.Context
	KubeConfig *rest.Config
}

type BackendProxy struct {
	proxy     *echo.Echo
	mainCtx   context.Context
	addr      string
	handlers  map[string]GatewayHandler
	k8sClient *kubernetes.Clientset
}

func (b *BackendProxyBuilder) Build() *BackendProxy {
	proxy := echo.New()

	backendProxy := &BackendProxy{
		proxy:     proxy,
		mainCtx:   b.MainCtx,
		addr:      b.Addr,
		handlers:  make(map[string]GatewayHandler),
		k8sClient: kubernetes.NewForConfigOrDie(b.KubeConfig),
	}
	// add handlers
	backendProxy.addHandlers(API_RESOURCES_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_RESOURCES_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_RAW_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_RAW_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_MD5_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_MD5_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PREVIEW_THUMB_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PREVIEW_THUMB_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PREVIEW_BIG_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PREVIEW_BIG_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PERMISSION_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PERMISSION_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_RESOURCES_CACHE_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_RESOURCES_CACHE_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_RAW_CACHE_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_RAW_CACHE_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_MD5_CACHE_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_MD5_CACHE_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PREVIEW_THUMB_CACHE_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PREVIEW_THUMB_CACHE_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PREVIEW_BIG_CACHE_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PREVIEW_BIG_CACHE_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PERMISSION_CACHE_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_PERMISSION_CACHE_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_CACHE_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_CACHE_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))

	proxy.Use(middleware.Recover())
	proxy.Use(middleware.Logger())
	proxy.Use(backendProxy.validate)
	proxy.Use(backendProxy.preHandle)

	config := middleware.DefaultProxyConfig
	config.Balancer = backendProxy
	proxy.Use(middleware.ProxyWithConfig(config))

	return backendProxy
}

func (p *BackendProxy) Start() error {
	return p.proxy.Start(p.addr)
}

func (p *BackendProxy) Shutdown() {
	klog.Info("gateway shutdown")
	if err := p.proxy.Shutdown(p.mainCtx); err != nil {
		klog.Errorln("shutdown error, ", err)
	}
}

func (p *BackendProxy) validate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		path := c.Request().URL.Path

		// deal for new cache method
		cachePrefixMap := map[string]string{
			API_RESOURCES_CACHE_PREFIX:     "resources",
			API_RAW_CACHE_PREFIX:           "raw",
			API_MD5_CACHE_PREFIX:           "md5",
			API_PREVIEW_THUMB_CACHE_PREFIX: "preview/thumb",
			API_PREVIEW_BIG_CACHE_PREFIX:   "preview/big",
			API_PERMISSION_CACHE_PREFIX:    "permission",
			API_PASTE_CACHE_PREFIX:         "paste",
		}

		for prefix, base := range cachePrefixMap {
			if path == prefix {
				path = fmt.Sprintf("/api/%s/AppData", base)
				break
			} else if path == prefix+"/" {
				path = fmt.Sprintf("/api/%s/AppData/", base)
				break
			} else {
				if strings.HasPrefix(path, prefix) {
					suffix := strings.TrimPrefix(path[len(prefix):], "/")

					parts := strings.SplitN(suffix, "/", 2)
					node := parts[0]
					remaining := ""
					if len(parts) > 1 {
						remaining = parts[1]
					}

					path = fmt.Sprintf("%s/%s/%s/AppData/%s", API_CACHE_PREFIX, node, base, remaining)
					break
				}
			}
		}

		if !regexp.MustCompile("^"+API_RESOURCES_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_RAW_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_MD5_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_PREVIEW_THUMB_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_PREVIEW_BIG_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_PERMISSION_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+UPLOADER_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+MEDIA_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_CACHE_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_BATCH_DELETE_PREFIX+".*").Match([]byte(path)) {
			klog.Errorln("unimplement api call, ", path)
			return c.String(http.StatusNotImplemented, "api not found")
		}

		if (strings.HasPrefix(path, API_PREFIX) || strings.HasPrefix(path, UPLOADER_PREFIX) || strings.HasPrefix(path, MEDIA_PREFIX)) &&
			!strings.HasPrefix(path, API_RESOURCES_PREFIX) &&
			!strings.HasPrefix(path, API_RAW_PREFIX) &&
			!strings.HasPrefix(path, API_MD5_PREFIX) &&
			!strings.HasPrefix(path, API_PREVIEW_THUMB_PREFIX) &&
			!strings.HasPrefix(path, API_PREVIEW_BIG_PREFIX) &&
			!strings.HasPrefix(path, API_PERMISSION_PREFIX) &&
			!strings.HasPrefix(path, API_CACHE_PREFIX) {
			return next(c)
		}

		if strings.HasPrefix(path, API_CACHE_PREFIX) {
			subPath := path[len(API_CACHE_PREFIX+"/"):]

			firstSlashIndex := strings.Index(subPath, "/")
			if firstSlashIndex != -1 {
				part := subPath[:firstSlashIndex]
				if strings.HasSuffix(part, "/") {
					part = part[:len(part)-1]
				}
				c.Request().URL.Path = API_PREFIX + subPath[firstSlashIndex:]
				c.Request().Header.Set(NODE_HEADER, part)
				klog.Info("Cache URL: ", c.Request().URL.Path)
				klog.Info("Cache Header: ", c.Request().Header.Get(NODE_HEADER))
			}
		}

		switch {
		case path != API_RESOURCES_PREFIX && path != API_RAW_PREFIX && path != API_MD5_PREFIX &&
			path != API_PREVIEW_THUMB_PREFIX && path != API_PREVIEW_BIG_PREFIX && path != API_PERMISSION_PREFIX:
			if _, ok := c.Request().Header[NODE_HEADER]; !ok {
				klog.Error("node info not found from header")
				return c.String(http.StatusBadRequest, "node not found")
			}
		}

		return next(c)
	}
}

func (p *BackendProxy) preHandle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		klog.Infof("Incoming request: %s", c.Request().URL.Path)
		if h, ok := p.handlers[c.Request().URL.Path]; ok {
			skip, err := h(c)
			if !skip {
				return err
			}
		}

		return next(c)
	}
}

func (p *BackendProxy) addHandlers(route string, handler GatewayHandler) {
	if route != "" {
		p.handlers[route] = handler
		klog.Infof("Added handler for route: %s", route)
		klog.Infof("Current handlers: %+v", p.handlers)
	}
}

func rewriteUrl(path string, pvc string, prefix string, hasFocusPrefix bool) string {
	if prefix == "" {
		dealPath := path
		focusPrefix := "/"
		if hasFocusPrefix {
			for _, fPrefix := range REWRITE_FOCUSED_PREFIXES {
				if strings.HasPrefix(path, fPrefix) {
					dealPath = strings.TrimPrefix(path, fPrefix)
					focusPrefix = fPrefix
					break
				}
			}
		}
		dealPath = strings.TrimPrefix(dealPath, "/")
		klog.Infof("Rewriting url for: %s with a focus prefix: %s", dealPath, focusPrefix)
		klog.Infof("pvc: %s", pvc)

		pathSplit := strings.Split(dealPath, "/")
		if len(pathSplit) < 2 {
			return ""
		}

		if pathSplit[0] != pvc {
			switch pathSplit[0] {
			case "external", "External":
				return focusPrefix + "External" + strings.TrimPrefix(dealPath, pathSplit[0])
			case "home", "Home":
				return focusPrefix + pvc + "/Home" + strings.TrimPrefix(dealPath, pathSplit[0])
			case "data", "Data", "application", "Application":
				return focusPrefix + pvc + "/Data" + strings.TrimPrefix(dealPath, pathSplit[0])
			}
		}
	} else {
		pathSuffix := strings.TrimPrefix(path, prefix)
		if strings.HasSuffix(prefix, "/cache") {
			prefix = strings.TrimSuffix(prefix, "/cache") + "/AppData"
		}
		if strings.HasPrefix(pathSuffix, "/"+pvc) {
			return path
		}
		return prefix + "/" + pvc + pathSuffix
	}
	return path
}

type PVCCache struct {
	proxy        *BackendProxy
	userPvcMap   map[string]string
	userPvcTime  map[string]time.Time
	cachePvcMap  map[string]string
	cachePvcTime map[string]time.Time
	mu           sync.RWMutex
}

func NewPVCCache(proxy *BackendProxy) *PVCCache {
	return &PVCCache{
		proxy:        proxy,
		userPvcMap:   make(map[string]string),
		userPvcTime:  make(map[string]time.Time),
		cachePvcMap:  make(map[string]string),
		cachePvcTime: make(map[string]time.Time),
	}
}

func (p *PVCCache) getFromCache(cached func() *string, fetch func() (string, error)) (string, error) {
	if c := func() *string {
		p.mu.RLock()
		defer p.mu.RUnlock()

		return cached()
	}(); c != nil {
		return *c, nil
	}

	return func() (string, error) {
		p.mu.Lock()
		defer p.mu.Unlock()

		if c := cached(); c != nil {
			return *c, nil
		}

		return fetch()
	}()
}

func (p *PVCCache) getUserPVCOrCache(bflName string) (string, error) {

	return p.getFromCache(
		func() *string {
			if val, ok := p.userPvcMap[bflName]; ok {
				if t, ok := p.userPvcTime[bflName]; ok && time.Since(t) <= 2*time.Minute {
					// p.userPvcTime[bflName] = time.Now()
					return &val
				}
			}

			return nil
		},
		func() (string, error) {
			userPvc, err := appdata.GetAnnotation(p.proxy.mainCtx, p.proxy.k8sClient, "userspace_pvc", bflName)
			if err != nil {
				return "", err
			}

			p.userPvcMap[bflName] = userPvc
			p.userPvcTime[bflName] = time.Now()
			return userPvc, nil
		},
	)

}

func (p *PVCCache) getCachePVCOrCache(bflName string) (string, error) {
	return p.getFromCache(
		func() *string {
			if val, ok := p.cachePvcMap[bflName]; ok {
				if t, ok := p.cachePvcTime[bflName]; ok && time.Since(t) <= 2*time.Minute {
					// p.cachePvcTime[bflName] = time.Now()
					return &val
				}
			}

			return nil
		},

		func() (string, error) {
			cachePvc, err := appdata.GetAnnotation(p.proxy.mainCtx, p.proxy.k8sClient, "appcache_pvc", bflName)
			if err != nil {
				return "", err
			}
			p.cachePvcMap[bflName] = cachePvc
			p.cachePvcTime[bflName] = time.Now()
			return cachePvc, nil
		},
	)
}

type BatchDeleteRequest struct {
	Dirents []string `json:"dirents"`
}

func (p *BackendProxy) Next(c echo.Context) *middleware.ProxyTarget {
	klog.Infof("Request Headers: %+v", c.Request().Header)

	node := c.Request().Header[NODE_HEADER]
	path := c.Request().URL.Path
	bfl := c.Request().Header[BFL_HEADER]
	bflName := ""
	if len(bfl) > 0 {
		bflName = bfl[0]
	}
	klog.Info("BFL_NAME: ", bflName)

	userPvc, err := PVCs.getUserPVCOrCache(bflName)
	if err != nil {
		klog.Info(err)
	} else {
		klog.Info("user-space pvc: ", userPvc)
	}

	cachePvc, err := PVCs.getCachePVCOrCache(bflName)
	if err != nil {
		klog.Info(err)
	} else {
		klog.Info("appcache pvc: ", cachePvc)
	}

	var host = ""

	if strings.HasPrefix(path, API_BATCH_DELETE_PREFIX) {
		var reqBody BatchDeleteRequest
		if err = json.NewDecoder(c.Request().Body).Decode(&reqBody); err != nil {
			klog.Info(err)
		}
		defer c.Request().Body.Close()

		modifiedDirents := make([]string, len(reqBody.Dirents))
		srcType := ""
		if len(reqBody.Dirents) > 0 {
			srcType, err = drives.ParsePathType(reqBody.Dirents[0], c.Request(), true, false)
			if err != nil {
				klog.Errorln(err)
				srcType = "Parse Error"
			}
			klog.Infoln("SRC_TYPE:", srcType)

			for i, dirent := range reqBody.Dirents {
				klog.Infof("dirents[%d]: %s", i, dirent)
				if srcType == drives.SrcTypeDrive || srcType == drives.SrcTypeData || srcType == drives.SrcTypeExternal {
					modifiedDirents[i] = rewriteUrl(dirent, userPvc, "", false)
				} else if srcType == drives.SrcTypeCache {
					if strings.HasPrefix(dirent, "/cache") {
						dirent = strings.TrimPrefix(dirent, "/cache")
						var dstNode string
						dstParts := strings.SplitN(strings.TrimPrefix(dirent, "/"), "/", 2)

						if len(dstParts) > 1 {
							dstNode = dstParts[0]
							if len(dirent) > len("/"+dstNode) {
								dirent = "/AppData" + dirent[len("/"+dstNode):]
							} else {
								dirent = "/AppData"
							}
							klog.Infoln("Node:", dstNode)
							klog.Infoln("New dirent:", dirent)
						} else if len(dstParts) > 0 {
							dstNode = dstParts[0]
							dirent = "/AppData"
							klog.Infoln("Node:", dstNode)
							klog.Infoln("New dirent:", dirent)
						}

						if len(node) == 0 {
							c.Request().Header.Set(NODE_HEADER, dstNode)
							node = c.Request().Header[NODE_HEADER]
						}
					} // only for cache for compatible
					modifiedDirents[i] = rewriteUrl(dirent, cachePvc, "/AppData", false)
				} else {
					modifiedDirents[i] = dirent
				}
				klog.Infof("modifiedDirents[%d]: %s", i, modifiedDirents[i])
			}
		}

		newReqBody := BatchDeleteRequest{Dirents: modifiedDirents}
		newBody, err := json.Marshal(newReqBody)
		if err != nil {
			klog.Errorln(err)
		}

		c.Request().Body = io.NopCloser(bytes.NewBuffer(newBody))
		c.Request().ContentLength = int64(len(newBody))
		c.Request().Header.Set("Content-Type", "application/json")
		if rc, ok := c.Request().Body.(io.ReadSeeker); ok {
			rc.Seek(0, io.SeekStart)
		}
	}

	if strings.HasPrefix(path, API_PASTE_PREFIX) {
		oldUrl := c.Request().URL.String()
		klog.Infoln("old url: ", oldUrl)

		parts := strings.Split(oldUrl, "?")
		src := parts[0]
		klog.Infoln("SRC:", src)

		parsedURL, _ := url.Parse(oldUrl)
		query := parsedURL.Query()
		dst := query.Get("destination")
		klog.Infoln("DST:", dst)

		srcType, err := drives.ParsePathType(strings.TrimPrefix(src, API_PASTE_PREFIX), c.Request(), false, false)
		if err != nil {
			klog.Errorln(err)
			srcType = "Parse Error"
		}
		dstType, err := drives.ParsePathType(dst, c.Request(), true, false)
		if err != nil {
			klog.Errorln(err)
			dstType = "Parse Error"
		}
		klog.Infoln("SRC_TYPE:", srcType)
		klog.Infoln("DST_TYPE:", dstType)

		if srcType == drives.SrcTypeDrive || srcType == drives.SrcTypeData || srcType == drives.SrcTypeExternal {
			src = rewriteUrl(src, userPvc, "", true)
		} else if srcType == drives.SrcTypeCache {
			src = rewriteUrl(src, cachePvc, API_PASTE_PREFIX+"/AppData", true)
		}

		if dstType == drives.SrcTypeDrive || dstType == drives.SrcTypeData || dstType == drives.SrcTypeExternal {
			dst = rewriteUrl(dst, userPvc, "", false)
			query.Set("destination", dst)
		} else if dstType == drives.SrcTypeCache {
			if strings.HasPrefix(dst, "/cache") {
				dst = strings.TrimPrefix(dst, "/cache")
				var dstNode string
				dstParts := strings.SplitN(strings.TrimPrefix(dst, "/"), "/", 2)

				if len(dstParts) > 1 {
					dstNode = dstParts[0]
					if len(dst) > len("/"+dstNode) {
						dst = "/AppData" + dst[len("/"+dstNode):]
					} else {
						dst = "/AppData"
					}
					klog.Infoln("Node:", dstNode)
					klog.Infoln("New dst:", dst)
				} else if len(dstParts) > 0 {
					dstNode = dstParts[0]
					dst = "/AppData"
					klog.Infoln("Node:", dstNode)
					klog.Infoln("New dst:", dst)
				}

				if len(node) == 0 {
					c.Request().Header.Set(NODE_HEADER, dstNode)
				}
			} // only for cache for compatible
			dst = rewriteUrl(dst, cachePvc, "/AppData", false)
			query.Set("destination", dst)
		}

		newURL := fmt.Sprintf("%s?%s", src, query.Encode())
		klog.Infoln("New WHOLE URL:", newURL)

		c.Request().URL.Path = src
		c.Request().URL.RawQuery = query.Encode()
	}

	if len(node) > 0 && !strings.HasPrefix(path, API_PASTE_PREFIX) {
		klog.Info("Node: ", node[0])
		if strings.HasPrefix(path, API_RESOURCES_PREFIX) {
			query := c.Request().URL.Query()
			dst := query.Get("destination")
			if dst != "" {
				if strings.HasPrefix(dst, "/cache") {
					dst = strings.TrimPrefix(dst, "/cache")
				} // only for cache for compatible
				dst = rewriteUrl(dst, cachePvc, "/AppData", false)
				dst = strings.Replace(dst, "/"+node[0], "", 1)
				query.Set("destination", dst)
				c.Request().URL.RawQuery = query.Encode()
			}

			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_RESOURCES_PREFIX, true)
		} else if strings.HasPrefix(path, API_RAW_PREFIX) {
			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_RAW_PREFIX, true)
		} else if strings.HasPrefix(path, API_MD5_PREFIX) {
			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_MD5_PREFIX, true)
		} else if strings.HasPrefix(path, API_PREVIEW_THUMB_PREFIX) {
			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_PREVIEW_THUMB_PREFIX, true)
		} else if strings.HasPrefix(path, API_PREVIEW_BIG_PREFIX) {
			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_PREVIEW_BIG_PREFIX, true)
		} else if strings.HasPrefix(path, API_PERMISSION_PREFIX) {
			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_PERMISSION_PREFIX, true)
		}
		host = appdata.GetAppDataServiceEndpoint(p.k8sClient, node[0])
		klog.Info("host: ", host)
		klog.Info("new path: ", c.Request().URL.Path)
	} else {
		klog.Info("Path: ", path)
		if strings.HasPrefix(path, API_PASTE_PREFIX) || strings.HasPrefix(path, API_BATCH_DELETE_PREFIX) {
			host = "127.0.0.1:8110"
		} else if strings.HasPrefix(path, API_PREFIX) {
			query := c.Request().URL.Query()
			dst := query.Get("destination")
			if dst != "" {
				if strings.HasPrefix(dst, "/cache") {
					dst = strings.TrimPrefix(dst, "/cache")
				} // only for cache for compatible
				klog.Infoln("DST:", dst)
				dst = rewriteUrl(dst, userPvc, "", false)
				query.Set("destination", dst)
				c.Request().URL.RawQuery = query.Encode()
			}
			c.Request().URL.Path = rewriteUrl(path, userPvc, "", true)
			host = "127.0.0.1:8110"
		} else if strings.HasPrefix(path, UPLOADER_PREFIX) {
			host = "127.0.0.1:40030"
		} else if strings.HasPrefix(path, MEDIA_PREFIX) {
			host = "127.0.0.1:9090"
		}
		klog.Info("host: ", host)
		klog.Info("new path: ", c.Request().URL.Path)
	}

	url, _ := url.ParseRequestURI("http://" + host + "/")
	klog.Info("Proxy URL: ", url)
	return &middleware.ProxyTarget{URL: url}
}

func (p *BackendProxy) AddTarget(*middleware.ProxyTarget) bool { return true }
func (p *BackendProxy) RemoveTarget(string) bool               { return true }

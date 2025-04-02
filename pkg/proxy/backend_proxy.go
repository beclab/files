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
	"context"
	"files/pkg/appdata"
	"files/pkg/drives"
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	API_RESOURCES_PREFIX     = "/api/resources/AppData"
	API_RAW_PREFIX           = "/api/raw/AppData"
	API_MD5_PREFIX           = "/api/md5/AppData"
	API_PREVIEW_THUMB_PREFIX = "/api/preview/thumb/AppData"
	API_PREVIEW_BIG_PREFIX   = "/api/preview/big/AppData"
	API_PERMISSION_PREFIX    = "/api/permission/AppData"
	NODE_HEADER              = "X-Terminus-Node"
	BFL_HEADER               = "X-Bfl-User"
	API_PREFIX               = "/api"
	UPLOADER_PREFIX          = "/upload"
	MEDIA_PREFIX             = "/videos"
	API_PASTE_PREFIX         = "/api/paste"
	API_CACHE_PREFIX         = "/api/cache"
)

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

		if !regexp.MustCompile("^"+API_RESOURCES_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_RAW_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_MD5_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_PREVIEW_THUMB_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_PREVIEW_BIG_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_PERMISSION_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+UPLOADER_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+MEDIA_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_CACHE_PREFIX+".*").Match([]byte(path)) {
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

func minWithNegativeOne(a, b int, aName, bName string) (int, string) {
	if a == -1 && b == -1 {
		return -1, ""
	}

	if a == -1 {
		return b, bName
	}
	if b == -1 {
		return a, aName
	}

	if a < b {
		return a, aName
	} else {
		return b, bName
	}
}

func rewriteUrl(path string, pvc string, prefix string) string {
	if prefix == "" {
		homeIndex := strings.Index(path, "/Home")
		applicationIndex := strings.Index(path, "/Application")
		splitIndex, splitName := minWithNegativeOne(homeIndex, applicationIndex, "/Home", "/Application")
		if splitIndex != -1 {
			firstHalf := path[:splitIndex]
			secondHalf := path[splitIndex:]
			klog.Info("firstHalf=", firstHalf)
			klog.Info("secondHalf=", secondHalf)

			if strings.HasSuffix(firstHalf, pvc) {
				return path
			}
			if splitName == "/Home" {
				return filepath.Join(firstHalf, pvc+secondHalf)
			} else {
				secondHalf = strings.TrimPrefix(path[splitIndex:], splitName)
				return filepath.Join(firstHalf, pvc+"/Data"+secondHalf)
			}
		}
	} else {
		pathSuffix := strings.TrimPrefix(path, prefix)
		if strings.HasPrefix(pathSuffix, "/"+pvc) {
			return path
		}
		return filepath.Join(prefix, pvc+pathSuffix)
	}
	return path
}

type PVCCache struct {
	proxy        *BackendProxy
	userPvcMap   map[string]string
	userPvcTime  map[string]time.Time
	cachePvcMap  map[string]string
	cachePvcTime map[string]time.Time
	mu           sync.Mutex
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

func (p *PVCCache) getUserPVCOrCache(bflName string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.userPvcMap[bflName]; ok {
		if t, ok := p.userPvcTime[bflName]; ok && time.Since(t) <= 2*time.Minute {
			p.userPvcTime[bflName] = time.Now()
			return val, nil
		}
	}

	userPvc, err := appdata.GetAnnotation(p.proxy.mainCtx, p.proxy.k8sClient, "userspace_pvc", bflName)
	if err != nil {
		return "", err
	}
	p.userPvcMap[bflName] = userPvc
	p.userPvcTime[bflName] = time.Now()
	return userPvc, nil
}

func (p *PVCCache) getCachePVCOrCache(bflName string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.cachePvcMap[bflName]; ok {
		if t, ok := p.cachePvcTime[bflName]; ok && time.Since(t) <= 2*time.Minute {
			p.cachePvcTime[bflName] = time.Now()
			return val, nil
		}
	}

	cachePvc, err := appdata.GetAnnotation(p.proxy.mainCtx, p.proxy.k8sClient, "appcache_pvc", bflName)
	if err != nil {
		return "", err
	}
	p.cachePvcMap[bflName] = cachePvc
	p.cachePvcTime[bflName] = time.Now()
	return cachePvc, nil
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

		//srcType := query.Get("src_type")
		//dstType := query.Get("dst_type")
		srcType, err := drives.ParsePathType(src, nil, false, false)
		if err != nil {
			klog.Errorln(err)
			srcType = "Parse Error"
		}
		dstType, err := drives.ParsePathType(dst, nil, true, false)
		if err != nil {
			klog.Errorln(err)
			dstType = "Parse Error"
		}
		klog.Infoln("SRC_TYPE:", srcType)
		klog.Infoln("DST_TYPE:", dstType)

		if srcType == drives.SrcTypeDrive {
			src = rewriteUrl(src, userPvc, "")
		} else if srcType == drives.SrcTypeCache {
			src = rewriteUrl(src, cachePvc, API_PASTE_PREFIX+"/AppData")
		}

		if dstType == drives.SrcTypeDrive {
			dst = rewriteUrl(dst, userPvc, "")
			query.Set("destination", dst)
		} else if dstType == drives.SrcTypeCache {
			dst = rewriteUrl(dst, cachePvc, "/AppData")
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
				dst = rewriteUrl(dst, cachePvc, "/AppData")
				dst = strings.Replace(dst, "/"+node[0], "", 1)
				query.Set("destination", dst)
				c.Request().URL.RawQuery = query.Encode()
			}

			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_RESOURCES_PREFIX)
		} else if strings.HasPrefix(path, API_RAW_PREFIX) {
			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_RAW_PREFIX)
		} else if strings.HasPrefix(path, API_MD5_PREFIX) {
			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_MD5_PREFIX)
		} else if strings.HasPrefix(path, API_PREVIEW_THUMB_PREFIX) {
			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_PREVIEW_THUMB_PREFIX)
		} else if strings.HasPrefix(path, API_PREVIEW_BIG_PREFIX) {
			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_PREVIEW_BIG_PREFIX)
		} else if strings.HasPrefix(path, API_PERMISSION_PREFIX) {
			c.Request().URL.Path = rewriteUrl(path, cachePvc, API_PERMISSION_PREFIX)
		}
		host = appdata.GetAppDataServiceEndpoint(p.k8sClient, node[0])
		klog.Info("host: ", host)
		klog.Info("new path: ", c.Request().URL.Path)
	} else {
		klog.Info("Path: ", path)
		if strings.HasPrefix(path, API_PASTE_PREFIX) {
			host = "127.0.0.1:8110"
		} else if strings.HasPrefix(path, API_PREFIX) {
			query := c.Request().URL.Query()
			dst := query.Get("destination")
			if dst != "" {
				klog.Infoln("DST:", dst)
				dst = rewriteUrl(dst, userPvc, "")
				query.Set("destination", dst)
				c.Request().URL.RawQuery = query.Encode()
			}
			c.Request().URL.Path = rewriteUrl(path, userPvc, "")
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

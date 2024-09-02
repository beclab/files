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
	"fmt"
	"github.com/Above-Os/files/gateway/pkg/appdata"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

const (
	API_RESOURCES_PREFIX = "/api/resources/AppData"
	API_RAW_PREFIX       = "/api/raw/AppData"
	NODE_HEADER          = "X-Terminus-Node"
	BFL_HEADER           = "X-Bfl-User"
	API_PREFIX           = "/api"
	UPLOADER_PREFIX      = "/upload"
	MEDIA_PREFIX         = "/videos"
	API_PASTE_PREFIX     = "/api/paste"
	API_CACHE_PREFIX     = "/api/cache"
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
	backendProxy.addHandlers(API_CACHE_PREFIX, backendProxy.listNodesOrNot(backendProxy.listNodes))
	backendProxy.addHandlers(API_CACHE_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.listNodes))
	//backendProxy.addHandlers(API_PREFIX, backendProxy.listNodesOrNot(backendProxy.apiHandler))
	//backendProxy.addHandlers(API_PREFIX+"/", backendProxy.listNodesOrNot(backendProxy.apiHandler))
	//backendProxy.addHandlers(UPLOADER_PREFIX, backendProxy.uploaderHandler)
	//backendProxy.addHandlers(UPLOADER_PREFIX+"/", backendProxy.uploaderHandler)
	//backendProxy.addHandlers(MEDIA_PREFIX, backendProxy.mediaHandler)
	//backendProxy.addHandlers(MEDIA_PREFIX+"/", backendProxy.mediaHandler)

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
		klog.Error("shutdown error, ", err)
	}
}

func (p *BackendProxy) validate(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		path := c.Request().URL.Path

		if !regexp.MustCompile("^"+API_RESOURCES_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_RAW_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+UPLOADER_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+MEDIA_PREFIX+".*").Match([]byte(path)) &&
			!regexp.MustCompile("^"+API_CACHE_PREFIX+".*").Match([]byte(path)) {
			klog.Error("unimplement api call, ", path)
			return c.String(http.StatusNotImplemented, "api not found")
		}

		if (strings.HasPrefix(path, API_PREFIX) || strings.HasPrefix(path, UPLOADER_PREFIX) || strings.HasPrefix(path, MEDIA_PREFIX)) &&
			!strings.HasPrefix(path, API_RESOURCES_PREFIX) &&
			!strings.HasPrefix(path, API_RAW_PREFIX) &&
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
		case path != API_RESOURCES_PREFIX && path != API_RAW_PREFIX:
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
		klog.Info("Incoming request: %s", c.Request().URL.Path)
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
		klog.Info("Added handler for route: %s", route)
		klog.Info("Current handlers: %+v", p.handlers)
	}
}

//func minWithNegativeOne(a, b int, aName, bName string) (int, string) {
//	if a == -1 && b == -1 {
//		return -1, ""
//	}
//
//	if a == -1 {
//		return b, bName
//	}
//	if b == -1 {
//		return a, aName
//	}
//
//	if a < b {
//		return a, aName
//	} else {
//		return b, bName
//	}
//}

func minWithNegativeOne(values []int, names []string) (int, string) {
	minValue := -1
	minName := ""

	for i, value := range values {
		if value != -1 {
			if minValue == -1 || value < minValue {
				minValue = value
				minName = names[i]
			}
		}
	}

	return minValue, minName
}

func rewriteUrl(path string, pvc string, prefix string) string {
	if prefix == "" {
		homeIndex := strings.Index(path, "/Home")
		applicationIndex := strings.Index(path, "/Application")
		externalIndex := strings.Index(path, "/external")
		splitIndex, splitName := minWithNegativeOne(
			[]int{homeIndex, applicationIndex, externalIndex},
			[]string{"/Home", "/Application", "/external"})
		if splitIndex != -1 {
			firstHalf := path[:splitIndex]
			secondHalf := path[splitIndex:]
			klog.Info("firstHalf=", firstHalf)
			klog.Info("secondHalf=", secondHalf)

			if strings.HasSuffix(firstHalf, pvc) {
				return path
			}
			if splitName == "/Home" {
				return firstHalf + "/" + pvc + secondHalf
			} else if splitName == "/Application" {
				secondHalf = strings.TrimPrefix(path[splitIndex:], splitName)
				return firstHalf + "/" + pvc + "/Data" + secondHalf
			} else if splitName == "/external" {
				return firstHalf + secondHalf
			}
		}
	} else {
		pathSuffix := strings.TrimPrefix(path, prefix)
		if strings.HasPrefix(pathSuffix, "/"+pvc) {
			return path
		}
		return prefix + "/" + pvc + pathSuffix
	}
	return path
}

type PVCCache struct {
	proxy       *BackendProxy
	userPvcMap  map[string]string
	cachePvcMap map[string]string
	mu          sync.Mutex
}

func NewPVCCache(proxy *BackendProxy) *PVCCache {
	return &PVCCache{
		proxy:       proxy,
		userPvcMap:  make(map[string]string),
		cachePvcMap: make(map[string]string),
	}
}

func (p *PVCCache) getUserPVCOrCache(bflName string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.userPvcMap[bflName]; ok {
		return val, nil
	}

	userPvc, err := appdata.GetAnnotation(p.proxy.mainCtx, p.proxy.k8sClient, "userspace_pvc", bflName)
	if err != nil {
		return "", err
	}
	p.userPvcMap[bflName] = userPvc
	return userPvc, nil
}

func (p *PVCCache) getCachePVCOrCache(bflName string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if val, ok := p.cachePvcMap[bflName]; ok {
		return val, nil
	}

	cachePvc, err := appdata.GetAnnotation(p.proxy.mainCtx, p.proxy.k8sClient, "appcache_pvc", bflName)
	if err != nil {
		return "", err
	}
	p.cachePvcMap[bflName] = cachePvc
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

	userPvc, err := PVCs.getUserPVCOrCache(bflName) // appdata.GetAnnotation(p.mainCtx, p.k8sClient, "userspace_pvc", bflName)
	if err != nil {
		klog.Info(err)
	} else {
		klog.Info("user-space pvc: ", userPvc)
	}

	cachePvc, err := PVCs.getCachePVCOrCache(bflName) // appdata.GetAnnotation(p.mainCtx, p.k8sClient, "appcache_pvc", bflName)
	if err != nil {
		klog.Info(err)
	} else {
		klog.Info("appcache pvc: ", cachePvc)
	}

	var host = ""

	if strings.HasPrefix(path, API_PASTE_PREFIX) {
		oldUrl := c.Request().URL.String()
		fmt.Println("old url: ", oldUrl)

		parts := strings.Split(oldUrl, "?")
		src := parts[0]
		fmt.Println("SRC:", src)

		parsedURL, _ := url.Parse(oldUrl)
		query := parsedURL.Query()
		dst := query.Get("destination")
		fmt.Println("DST:", dst)

		srcType := query.Get("src_type")
		dstType := query.Get("dst_type")
		fmt.Println("SRC_TYPE:", srcType)
		fmt.Println("DST_TYPE:", dstType)

		if srcType == "drive" {
			src = rewriteUrl(src, userPvc, "")
		} else if srcType == "cache" {
			src = rewriteUrl(src, cachePvc, API_PASTE_PREFIX+"/AppData")
		} else if srcType == "sync" {
			src = src
		}

		if dstType == "drive" {
			dst = rewriteUrl(dst, userPvc, "")
			query.Set("destination", dst)
		} else if dstType == "cache" {
			dst = rewriteUrl(dst, cachePvc, "/AppData")
			query.Set("destination", dst)
		} else if dstType == "sync" {
			dst = dst
		}

		newURL := fmt.Sprintf("%s?%s", src, query.Encode())
		fmt.Println("New WHOLE URL:", newURL)

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
				fmt.Println("DST:", dst)
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

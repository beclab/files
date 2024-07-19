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
)

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
			!regexp.MustCompile("^"+MEDIA_PREFIX+".*").Match([]byte(path)) {
			klog.Error("unimplement api call, ", path)
			return c.String(http.StatusNotImplemented, "api not found")
		}

		if (strings.HasPrefix(path, API_PREFIX) || strings.HasPrefix(path, UPLOADER_PREFIX) || strings.HasPrefix(path, MEDIA_PREFIX)) &&
			!strings.HasPrefix(path, API_RESOURCES_PREFIX) &&
			!strings.HasPrefix(path, API_RAW_PREFIX) {
			return next(c)
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

func rewriteUrl(path string, pvc string) string {
	homeIndex := strings.Index(path, "/Home")
	applicationIndex := strings.Index(path, "/Application")
	splitIndex, splitName := minWithNegativeOne(homeIndex, applicationIndex, "/Home", "/Application")
	if splitIndex != -1 {
		if splitName == "/Home" {
			firstHalf := path[:splitIndex]
			secondHalf := path[splitIndex:]
			return firstHalf + "/" + pvc + secondHalf
		} else {
			firstHalf := path[:splitIndex]
			secondHalf := strings.TrimPrefix(path[splitIndex:], splitName)
			return firstHalf + "/" + pvc + "/Data" + secondHalf
		}
	}
	return path
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

	userPvc, err := appdata.GetAnnotation(p.mainCtx, p.k8sClient, "", "userspace_pvc", bflName)
	if err != nil {
		klog.Info(err)
	} else {
		klog.Info("user-space pvc: ", userPvc)
	}

	cachePvc, err := appdata.GetAnnotation(p.mainCtx, p.k8sClient, "", "appcache_pvc", bflName)
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

		//if srcType != "sync" {
		//	src = rewriteUrl(src, userPvc)
		//}
		if dstType == "drive" {
			dst = rewriteUrl(dst, userPvc)
			query.Set("destination", dst)
		} else if srcType == "cache" && dstType == "cache" {
			pathSuffix := strings.TrimPrefix(dst, "/AppData")
			query.Set("destination", "/AppData/"+cachePvc+pathSuffix)
		}

		newURL := fmt.Sprintf("%s?%s", src, query.Encode())
		fmt.Println("New WHOLE URL:", newURL)

		c.Request().URL.Path = src
		c.Request().URL.RawQuery = query.Encode()
	}

	if len(node) > 0 && !strings.HasPrefix(path, API_PASTE_PREFIX) {
		klog.Info("Node: ", node[0])
		if strings.HasPrefix(path, API_RESOURCES_PREFIX) {
			pathSuffix := strings.TrimPrefix(path, API_RESOURCES_PREFIX)
			c.Request().URL.Path = API_RESOURCES_PREFIX + "/" + cachePvc + pathSuffix
		} else if strings.HasPrefix(path, API_RAW_PREFIX) {
			pathSuffix := strings.TrimPrefix(path, API_RAW_PREFIX)
			c.Request().URL.Path = API_RAW_PREFIX + "/" + cachePvc + pathSuffix
		}
		//} else if strings.HasPrefix(path, API_PASTE_PREFIX) {
		//pathSuffix := strings.TrimPrefix(path, API_PASTE_PREFIX+"/AppData")
		//c.Request().URL.Path = API_PASTE_PREFIX + "/AppData/" + cachePvc + pathSuffix
		//}
		host = appdata.GetAppDataServiceEndpoint(node[0])
		klog.Info("host: ", host)
		klog.Info("new path: ", c.Request().URL.Path)
	} else {
		klog.Info("Path: ", path)
		if strings.HasPrefix(path, API_PASTE_PREFIX) {
			host = "127.0.0.1:8110"
		} else if strings.HasPrefix(path, API_PREFIX) {
			//homeIndex := strings.Index(path, "/Home")
			//applicationIndex := strings.Index(path, "/Application")
			//splitIndex, splitName := minWithNegativeOne(homeIndex, applicationIndex, "/Home", "/Application")
			//if splitIndex != -1 {
			//	if splitName == "/Home" {
			//		firstHalf := path[:splitIndex]
			//		secondHalf := path[splitIndex:]
			//		c.Request().URL.Path = firstHalf + "/" + userPvc + secondHalf
			//	} else {
			//		firstHalf := path[:splitIndex]
			//		secondHalf := strings.TrimPrefix(path[splitIndex:], splitName)
			//		c.Request().URL.Path = firstHalf + "/" + userPvc + "/Data" + secondHalf
			//	}
			//}
			c.Request().URL.Path = rewriteUrl(path, userPvc)
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

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
	API_PREFIX           = "/api"
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
	backendProxy.addHandlers(API_PREFIX, backendProxy.apiHandler)
	backendProxy.addHandlers(API_PREFIX+"/", backendProxy.apiHandler)

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
			!regexp.MustCompile("^"+API_PREFIX+".*").Match([]byte(path)) {
			klog.Error("unimplement api call, ", path)
			return c.String(http.StatusNotImplemented, "api not found")
		}

		if strings.HasPrefix(path, API_PREFIX) &&
			!strings.HasPrefix(path, API_RESOURCES_PREFIX) &&
			!strings.HasPrefix(path, API_RAW_PREFIX) {
			// return next(c)
		} else {
			switch {
			case path != API_RESOURCES_PREFIX && path != API_RAW_PREFIX:
				if _, ok := c.Request().Header[NODE_HEADER]; !ok {
					klog.Error("node info not found from header")
					return c.String(http.StatusBadRequest, "node not found")
				}
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

func (p *BackendProxy) Next(c echo.Context) *middleware.ProxyTarget {
	node := c.Request().Header[NODE_HEADER]
	var host = ""
	if len(node) > 0 {
		host = appdata.GetAppDataServiceEndpoint(node[0])
	} else {
		host = "127.0.0.1:8110"
	}

	url, _ := url.ParseRequestURI("http://" + host + "/")
	return &middleware.ProxyTarget{URL: url}
}

func (p *BackendProxy) AddTarget(*middleware.ProxyTarget) bool { return true }
func (p *BackendProxy) RemoveTarget(string) bool               { return true }

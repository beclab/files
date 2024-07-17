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
	"net/http"

	"github.com/labstack/echo/v4"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (p *BackendProxy) listNodes(c echo.Context) (bool, error) {
	klog.Info("list terminus node")
	nodes, err := p.k8sClient.CoreV1().Nodes().List(c.Request().Context(), metav1.ListOptions{})
	if err != nil {
		klog.Error("list nodes error, ", err)
		return false, err
	}

	return false, c.JSON(http.StatusOK, echo.Map{
		"code": 200,
		"data": nodes.Items,
	})
}

func (p *BackendProxy) apiHandler(c echo.Context) (bool, error) {
	req := c.Request()

	// 创建新的 HTTP 请求
	proxyReq, err := http.NewRequestWithContext(c.Request().Context(), req.Method, "http://127.0.0.1:8110"+req.RequestURI, req.Body)
	if err != nil {
		return false, err
	}

	// 复制请求头
	for k, v := range req.Header {
		proxyReq.Header[k] = v
	}

	// 执行代理请求
	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// 将响应内容写回给客户端
	return false, c.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
}

func (p *BackendProxy) uploaderHandler(c echo.Context) (bool, error) {
	req := c.Request()

	// 创建新的 HTTP 请求
	proxyReq, err := http.NewRequestWithContext(c.Request().Context(), req.Method, "http://127.0.0.1:40030"+req.RequestURI, req.Body)
	if err != nil {
		return false, err
	}

	// 复制请求头
	for k, v := range req.Header {
		proxyReq.Header[k] = v
	}

	// 执行代理请求
	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// 将响应内容写回给客户端
	return false, c.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
}

func (p *BackendProxy) mediaHandler(c echo.Context) (bool, error) {
	req := c.Request()

	// 创建新的 HTTP 请求
	proxyReq, err := http.NewRequestWithContext(c.Request().Context(), req.Method, "http://127.0.0.1:9090"+req.RequestURI, req.Body)
	if err != nil {
		return false, err
	}

	// 复制请求头
	for k, v := range req.Header {
		proxyReq.Header[k] = v
	}

	// 执行代理请求
	resp, err := http.DefaultClient.Do(proxyReq)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// 将响应内容写回给客户端
	return false, c.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
}

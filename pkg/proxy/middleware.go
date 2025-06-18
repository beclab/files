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
	"files/pkg/models"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"k8s.io/klog/v2"
)

func (p *BackendProxy) listNodesOrNot(listFunc GatewayHandler) GatewayHandler {
	return func(c echo.Context) (next bool, err error) {
		if _, ok := c.Request().Header[NODE_HEADER]; ok {
			return true, nil
		}
		return listFunc(c)
	}
}

func (p *BackendProxy) nextListHandle(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var path = c.Request().URL.Path
		if !strings.HasPrefix(path, "/api/resources") {
			return next(c)
		}

		var users = c.Request().Header[BFL_HEADER]

		if users == nil || len(users) == 0 {
			return c.String(http.StatusBadRequest, "users not found")
		}

		var owner string
		if len(users) > 0 {
			owner = users[0]
		}

		klog.Infof("Owner: %s, Path: %s", owner, path)

		userPvc, err := PVCs.getUserPVCOrCache(owner)
		if err != nil {
			return c.String(http.StatusBadRequest, "users not found")
		}

		cachePvc, err := PVCs.getCachePVCOrCache(owner)
		if err != nil {
			return c.String(http.StatusBadRequest, "users not found")
		}

		var data = models.PathFormatter(path)
		var param = &models.FileParam{
			Query:    c.Request().URL.Query(),
			Header:   c.Request().Header,
			UserPvc:  userPvc,
			CachePvc: cachePvc,
		}

		param.ConvertToBackendUrl(data)
		paramdata := param.Json()
		klog.Infof("file param: %s", paramdata)

		c.Request().Header.Add("GATEWAY_FILE_PARAM", paramdata)

		return next(c)
	}
}

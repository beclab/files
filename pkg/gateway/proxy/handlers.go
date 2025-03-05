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

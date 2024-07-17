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
	"github.com/labstack/echo/v4"
	"k8s.io/klog/v2"
	"reflect"
	"runtime"
)

func getFunctionName(f interface{}) string {
	fptr, ok := f.(func())
	if !ok {
		return "Not a function"
	}

	funcsym := runtime.FuncForPC(reflect.ValueOf(fptr).Pointer())
	if funcsym == nil {
		return "Unknown"
	}

	return funcsym.Name()
}

func (p *BackendProxy) listNodesOrNot(listFunc GatewayHandler) GatewayHandler {
	klog.Info("listNodesOrNot Func Name: ", getFunctionName(listFunc))
	return func(c echo.Context) (next bool, err error) {
		if _, ok := c.Request().Header[NODE_HEADER]; ok {
			return true, nil
		}
		return listFunc(c)
	}
}

// Package usage registers the /api/usage/*path route.
package usage

import (
	usage "files/pkg/hertz/biz/handler/api/usage"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func Register(r *server.Hertz) {
	root := r.Group("/", rootMw()...)
	{
		_api := root.Group("/api", _apiMw()...)
		{
			_usage := _api.Group("/usage", _usageMw()...)
			_usage.GET("/*path", append(_usageMethodMw(), usage.UsageMethod)...)
		}
	}
}

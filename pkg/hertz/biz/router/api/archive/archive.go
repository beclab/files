// Package archive registers the /api/archive/:node/... routes.
//
// Unlike the auto-generated routers in sibling packages, this one is
// hand-written: archive's preview endpoints stream NDJSON / raw bytes
// which the thrift IDL cannot express directly, so we register all
// four handlers (compress / extract / entries / entry) explicitly.
package archive

import (
	archhandler "files/pkg/hertz/biz/handler/api/archive"

	"github.com/cloudwego/hertz/pkg/app/server"
)

func Register(r *server.Hertz) {
	root := r.Group("/", rootMw()...)
	{
		api := root.Group("/api", _apiMw()...)
		{
			arc := api.Group("/archive", _archiveMw()...)
			{
				node := arc.Group("/:node", _nodeMw()...)
				node.POST("/compress", append(_compressMethodMw(), archhandler.CompressMethod)...)
				node.POST("/extract", append(_extractMethodMw(), archhandler.ExtractMethod)...)
				node.GET("/entries", append(_entriesMethodMw(), archhandler.EntriesMethod)...)
				node.GET("/entry", append(_entryMethodMw(), archhandler.EntryMethod)...)
			}
		}
	}
}

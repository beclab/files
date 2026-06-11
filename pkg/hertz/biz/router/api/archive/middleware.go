package archive

import (
	bizhandler "files/pkg/hertz/biz/handler"

	"github.com/cloudwego/hertz/pkg/app"
)

func rootMw() []app.HandlerFunc            { return nil }
func _apiMw() []app.HandlerFunc            { return nil }
func _archiveMw() []app.HandlerFunc        { return nil }
func _nodeMw() []app.HandlerFunc           { return []app.HandlerFunc{bizhandler.NodeGuard()} }
func _compressMethodMw() []app.HandlerFunc { return nil }
func _extractMethodMw() []app.HandlerFunc  { return nil }
func _entriesMethodMw() []app.HandlerFunc  { return nil }
func _entryMethodMw() []app.HandlerFunc    { return nil }

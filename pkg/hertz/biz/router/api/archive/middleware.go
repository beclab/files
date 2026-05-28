package archive

import "github.com/cloudwego/hertz/pkg/app"

func rootMw() []app.HandlerFunc          { return nil }
func _apiMw() []app.HandlerFunc          { return nil }
func _archiveMw() []app.HandlerFunc      { return nil }
func _nodeMw() []app.HandlerFunc         { return nil }
func _compressMethodMw() []app.HandlerFunc { return nil }
func _extractMethodMw() []app.HandlerFunc  { return nil }
func _entriesMethodMw() []app.HandlerFunc  { return nil }
func _entryMethodMw() []app.HandlerFunc    { return nil }

package main

import (
	"files/cmd/backend/app"
	"files/pkg/media/api"
        "files/pkg/media/service"
)

func main() {
	go func() {
		service.Init()

		api.StartHttpServer()
	}()

	app.Execute()
}

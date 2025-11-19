package main

import (
	"files/cmd/backend/app"
	"files/pkg/media/service"
)

func main() {
	go func() {
		service.Init()
	}()

	app.Execute()
}

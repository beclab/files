package main

import (
	"files/cmd/backend/app"
	"files/pkg/media/service"
)

func main() {
	service.Init()
	app.Execute()
}

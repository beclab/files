package common

import "files/pkg/settings"

type Data struct {
	Server *settings.Server
	Raw    interface{}
}

package settings

import (
	"strings"
)

// Server specific settings.
type Server struct {
	Root                  string `json:"root"`
	BaseURL               string `json:"baseURL"`
	Socket                string `json:"socket"`
	TLSKey                string `json:"tlsKey"`
	TLSCert               string `json:"tlsCert"`
	Port                  string `json:"port"`
	Address               string `json:"address"`
	Log                   string `json:"log"`
	EnableThumbnails      bool   `json:"enableThumbnails"`
	ResizePreview         bool   `json:"resizePreview"`
	EnableExec            bool   `json:"enableExec"`
	TypeDetectionByHeader bool   `json:"typeDetectionByHeader"`
	AuthHook              string `json:"authHook"`
}

func NewDefaultServer() *Server {
	return &Server{
		Root:                  "/srv",
		BaseURL:               "",
		Socket:                "",
		TLSKey:                "",
		TLSCert:               "",
		Port:                  "8110",
		Address:               "",
		Log:                   "stdout",
		EnableThumbnails:      true,
		ResizePreview:         false,
		EnableExec:            false,
		TypeDetectionByHeader: true,
		AuthHook:              "",
	}
}

// Clean cleans any variables that might need cleaning.
func (s *Server) Clean() {
	s.BaseURL = strings.TrimSuffix(s.BaseURL, "/")
}

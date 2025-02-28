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

// Clean cleans any variables that might need cleaning.
func (s *Server) Clean() {
	s.BaseURL = strings.TrimSuffix(s.BaseURL, "/")
}

// GenerateKey generates a key of 512 bits.
//func GenerateKey() ([]byte, error) {
//	b := make([]byte, 64) //nolint:gomnd
//	_, err := rand.Read(b)
//	// Note that err == nil only if we read len(b) bytes.
//	if err != nil {
//		return nil, err
//	}
//
//	return b, nil
//}

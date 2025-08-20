package common

var (
	ServeAddr = "http://127.0.0.1:5572"
	ServeHost = "127.0.0.1"
)

type ErrorMessage struct {
	Error  string      `json:"error"`
	Input  interface{} `json:"input"`
	Path   string      `json:"path"`
	Status int         `json:"status"`
}

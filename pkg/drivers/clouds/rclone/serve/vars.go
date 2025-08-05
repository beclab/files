package serve

var (
	ServeStartPort int = 17200
)

var (
	StartServePath = "serve/start"
	StopServePath  = "serve/stop"
	ListServePath  = "serve/list"
)

type Serve struct {
	Id         string `json:"id"` // like http-4fecd788
	Name       string `json:"name"`
	Type       string `json:"type"`
	Fs         string `json:"fs"`
	Addr       string `json:"addr"`
	BufferSize string `json:"buffer_size"`
	// BaseUrl      string `json:"baseurl"`
	VfsCacheMode string `json:"vfs_cache_mode"`
	Port         int    `json:"port"`
}

// Start Resp
type StartServeResp struct {
	Id   string `json:"id"`
	Addr string `json:"addr"`
}

// List Resp
type ServeListResp struct {
	List []*ServeListItem `json:"list"`
}

type ServeListItem struct {
	Id     string               `json:"id"`
	Addr   string               `json:"addr"`
	Params *ServeListItemParams `json:"params"`
}

type ServeListItemParams struct {
	Name         string `json:"name"`
	Addr         string `json:"addr"`
	BufferSize   string `json:"buffer_size"`
	Fs           string `json:"fs"`
	Type         string `json:"type"`
	VfsCachemode string `json:"vfs_cache_mode"`
	Port         int    `json:"port"`
}

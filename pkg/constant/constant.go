package constant

import "os"

var (
	FreeLimit float64 = 85.00
)

const (
	ROOT_PREFIX     = "/data"
	CACHE_PREFIX    = "/appcache"
	EXTERNAL_PREFIX = "/data/External"
)

const (
	REQUEST_HEADER_OWNER = "X-Bfl-User"
	REQUEST_HEADER_NODE  = "X-Terminus-Node"
	REQUEST_HEADER_TOKEN = "Terminus-Nonce"
)

var (
	OlaresdHost    = os.Getenv("TERMINUSD_HOST")
	ExternalPrefix = os.Getenv("EXTERNAL_PREFIX")
	NodeName       = os.Getenv("NODE_NAME")
)

const (
	Posix       = "posix"
	Drive       = "drive"
	Home        = "home"
	Data        = "data"
	Cache       = "cache"
	External    = "external"
	Internal    = "internal"
	Usb         = "usb"
	Hdd         = "hdd"
	Smb         = "smb"
	Sync        = "sync"
	Cloud       = "cloud"
	AwsS3       = "awss3"
	GoogleDrive = "google"
	DropBox     = "dropbox"
	TencentCos  = "tencent"
)

const (
	Text  = "text"
	Image = "image"
)

const (
	Pending   = "pending"
	Running   = "running"
	Failed    = "failed"
	Cancelled = "cancelled"
	Completed = "completed"
)

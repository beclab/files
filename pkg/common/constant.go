package common

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
	Local       = "local"
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

	RcloneTypeLocal   = "local"
	RcloneTypeS3      = "s3"
	RcloneTypeDrive   = "drive"
	RcloneTypeDropbox = "dropbox"

	ProviderAWS        = "AWS"
	ProviderTencentCOS = "TencentCOS"
)

const (
	Pending   = "pending"
	Running   = "running"
	Failed    = "failed"
	Canceled  = "canceled"
	Completed = "completed"

	ActionCopy   = "copy"
	ActionMove   = "move"
	ActionUpload = "upload"
)

var (
	DefaultLocalRootPath             = "/data/"
	DefaultKeepFileName              = ".keep"
	DefaultSyncUploadToCloudTempPath = "/files_cache/.downloadstemp"
	DefaultUploadTempDir             = ".uploadstemp"
	DefaultUploadToCloudTempPath     = "/files_cache/" + DefaultUploadTempDir
)

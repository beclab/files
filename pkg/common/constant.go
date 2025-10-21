package common

import "os"

var (
	FreeLimit float64 = 85.00
)

const (
	ROOT_PREFIX     = "/data"
	CACHE_PREFIX    = "/appcache"
	EXTERNAL_PREFIX = "/data/External"

	DefaultNamespace              = "os-framework"
	DefaultServiceAccount         = "os-internal"
	DefaultIntegrationProviderUrl = "http://integration-provider-svc.os-protected:28080"

	EnvIntegrationDebug = "FILES_INTEGRATION_DEBUG"
)

const (
	REQUEST_HEADER_OWNER         = "X-Bfl-User"
	REQUEST_HEADER_AUTHORIZATION = "Authorization"
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
	Space       = "space"

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
	Paused    = "paused"
	Resumed   = "resumed"

	ActionCopy   = "copy"
	ActionMove   = "move"
	ActionUpload = "upload"
)

var (
	DefaultLocalRootPath             = "/data/"
	DefaultLocalFileCachePath        = "/files_cache/"
	DefaultKeepFileName              = ".keep"
	DefaultSyncUploadToCloudTempPath = DefaultLocalFileCachePath + ".downloadstemp"
	DefaultUploadTempDir             = ".uploadstemp"
	DefaultUploadToCloudTempPath     = DefaultLocalFileCachePath + DefaultUploadTempDir

	CacheBuffer = "buffer"
	CacheThumb  = "thumb"
)

var (
	multiExts = []string{
		".pb.go", ".pb.cc", ".pb.h", ".user.js", ".test.js", ".spec.js", ".min.js", ".min.css",
		".tar.gz", ".tar.bz2", ".tar.xz", ".tar.Z", ".tar.lz", ".tar.lzma", ".tar.lzo", ".tar.sz", ".tar.zst", ".tar.br",
		".cpio.gz", ".cpio.bz2", ".cpio.xz",
		".csv.gz", ".json.gz", ".xml.gz", ".log.gz", ".tsv.gz", ".sqlite.gz",
		".d.ts",
	}
)

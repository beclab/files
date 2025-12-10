package common

import "os"

var (
	FreeLimit float64 = 85.00
)

const (
	ROOT_PREFIX     = "/data"
	CACHE_PREFIX    = "/appcache"
	CACHE_ALIAS     = "/AppData"
	EXTERNAL_PREFIX = "/data/External"

	SERVER_HOST = "127.0.0.1:8080"

	SambaConfTemplatePath = "/etc/samba/smb.conf"

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
	OlaresdHost      = os.Getenv("TERMINUSD_HOST")
	ExternalPrefix   = os.Getenv("EXTERNAL_PREFIX")
	NodeName         = os.Getenv("NODE_NAME")
	DebugIntegration = os.Getenv("DEBUG_INTEGRATION")
)

const (
	Local       = "local"
	Posix       = "posix"
	Drive       = "drive"
	Home        = "Home"
	Data        = "Data"
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
	Share       = "share"
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
	ShareTypeInternal = "internal"
	ShareTypeExternal = "external"
	ShareTypeSMB      = "smb"

	ErrorMessageDirNotExists               = "Directory not exist."
	ErrorMessageShareNotExists             = "This share no longer exists. The link may have been deleted."
	ErrorMessageShareTypeInvalid           = "Share type invalid."
	ErrorMessagePathInvalid                = "Path invalid."
	ErrorMessageOwnerNotFound              = "Owner not found."
	ErrorMessageWrongShare                 = "Wrong share."
	ErrorMessagePasteWrongSourceShare      = "Invalid source share path to copy."
	ErrorMessagePasteSourceExpired         = "The sharing source path to copy has expired."
	ErrorMessagePasteWrongDestinationShare = "Invalid destination share path to copy."
	ErrorMessagePasteDestinationExpired    = "The sharing destination path to copy has expired."
	ErrorMessageTokenExpired               = "Token expired."
	ErrorMessageTokenInvalid               = "Token is invalid."
	ErrorMessageLinkExpired                = "Link expired."
	ErrorMessageGetTokenError              = "GetToken failed."
	ErrorMessagePermissionDenied           = "Permission denied."
	ErrorMessageUserExists                 = "User already exists or is used by another account."
	ErrorMessageShareExists                = "Share exists."
	ErrorMesssageSambaPasswordInvalid      = "Samba share password invalid."
	ErrorMessageWrongPassword              = "Wrong password. Please check the password and try again."
	ErrorMessageInternalPathExists         = "Share for a path can be only one at a time."
	ErrorMessageSyncNotSupport             = "Sync not support."
	ErrorMessageNoSpace                    = "Insufficient space on the disk, usage exceeds 85%."

	CodeLinkExpired  = 559
	CodeTokenExpired = 569
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

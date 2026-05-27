package common

import (
	"os"
	"strings"
)

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

	ActionCopy            = "copy"
	ActionMove            = "move"
	ActionUpload          = "upload"
	ActionUploadFinalize  = "upload_finalize"
	ActionCompress        = "compress"
	ActionExtract         = "extract"

	AsyncFinalizeThreshold int64 = 2 * 1024 * 1024 * 1024 // 2GB
)

const (
	ArchiveFormatZip    = "zip"
	ArchiveFormat7z     = "7z"
	ArchiveFormatTar    = "tar"
	ArchiveFormatTarGz  = "tar.gz"
	ArchiveFormatTgz    = "tgz"
	ArchiveFormatTarBz2 = "tar.bz2"
	ArchiveFormatTarXz  = "tar.xz"
	ArchiveFormatGzip   = "gzip"
	ArchiveFormatBzip2  = "bzip2"
	ArchiveFormatXz     = "xz"

	ArchiveConflictRename    = "rename"
	ArchiveConflictOverwrite = "overwrite"
	ArchiveConflictSkip      = "skip"
)

var (
	// ArchiveFormatsWrite lists every format the compress endpoint
	// accepts; all of them are produced by the 7z CLI backend.
	ArchiveFormatsWrite = []string{
		ArchiveFormatZip, ArchiveFormat7z, ArchiveFormatTar,
		ArchiveFormatTarGz, ArchiveFormatTgz, ArchiveFormatTarBz2, ArchiveFormatTarXz,
		ArchiveFormatGzip, ArchiveFormatBzip2, ArchiveFormatXz,
	}
	// ArchiveFormatsRead lists every format the extract / preview
	// endpoints accept.
	ArchiveFormatsRead = []string{
		ArchiveFormatZip, ArchiveFormat7z, ArchiveFormatTar,
		ArchiveFormatTarGz, ArchiveFormatTgz, ArchiveFormatTarBz2, ArchiveFormatTarXz,
		ArchiveFormatGzip, ArchiveFormatBzip2, ArchiveFormatXz,
	}
	// ArchiveFormatsWithPassword restricts password-protected archives
	// to formats that natively support encryption (zip / 7z).
	ArchiveFormatsWithPassword = []string{ArchiveFormatZip, ArchiveFormat7z}
	// ArchiveFormatsWithVolume restricts multi-volume archives to
	// formats that natively support splitting (zip / 7z).
	ArchiveFormatsWithVolume = []string{ArchiveFormatZip, ArchiveFormat7z}
	// ArchiveFormatsStdlibRead lists formats that the reader package
	// can handle without spawning a 7z subprocess.
	ArchiveFormatsStdlibRead = []string{
		ArchiveFormatZip, ArchiveFormatTar, ArchiveFormatTarGz, ArchiveFormatTgz,
	}
	// PosixFileTypes is the whitelist of storage types on which archive
	// operations are supported.
	PosixFileTypes = []string{Drive, Cache, External, Internal, Usb, Hdd, Smb}
)

// ArchiveFormatFromName infers the archive format from a file name's
// suffix; returns "" when the name has no known extension.
func ArchiveFormatFromName(name string) string {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".tar.gz") {
		return ArchiveFormatTarGz
	}
	if strings.HasSuffix(lower, ".tgz") {
		return ArchiveFormatTgz
	}
	if strings.HasSuffix(lower, ".tar.bz2") {
		return ArchiveFormatTarBz2
	}
	if strings.HasSuffix(lower, ".tar.xz") {
		return ArchiveFormatTarXz
	}
	if strings.HasSuffix(lower, ".zip") {
		return ArchiveFormatZip
	}
	if strings.HasSuffix(lower, ".7z") {
		return ArchiveFormat7z
	}
	if strings.HasSuffix(lower, ".tar") {
		return ArchiveFormatTar
	}
	if strings.HasSuffix(lower, ".gz") {
		return ArchiveFormatGzip
	}
	if strings.HasSuffix(lower, ".bz2") {
		return ArchiveFormatBzip2
	}
	if strings.HasSuffix(lower, ".xz") {
		return ArchiveFormatXz
	}
	return ""
}

var (
	DefaultLocalRootPath             = "/data/"
	DefaultLocalFileCachePath        = "/files_cache/"
	DefaultKeepFileName              = ".keep"
	DefaultSyncUploadToCloudTempPath = DefaultLocalFileCachePath + ".downloadstemp"
	DefaultUploadTempDir             = ".uploadstemp"
	DefaultUploadToCloudTempPath     = DefaultLocalFileCachePath + DefaultUploadTempDir

	CacheBuffer = "buffer"
	CacheThumb  = "thumb"
	CloudCache  = "cloud_cache"
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
	ErrorMessagePasteSrcNotExists          = "Source path to copy does not exist or is not accessible."
	ErrorMessageTokenExpired               = "Token expired."
	ErrorMessageTokenInvalid               = "Token is invalid."
	ErrorMessageLinkExpired                = "Link expired."
	ErrorMessageGetTokenError              = "GetToken failed."
	ErrorMessagePermissionDenied           = "Permission denied."
	ErrorMessageUserExists                 = "User already exists or is used by another account."
	ErrorMessageShareExists                = "Share exists."
	ErrorMesssageSambaPasswordInvalid      = "Samba share password invalid."
	ErrorMessageSmbUserNameLength          = "SMB user name length must be between 6 and 16 characters."
	ErrorMessageSmbUserNameInvalid         = "SMB user name must start with a lowercase letter or underscore, may contain only lowercase letters, digits, underscores or hyphens, and must not end with a hyphen."
	ErrorMessageSmbUserNameReserved        = "SMB user name is reserved by the system."
	ErrorMessageSmbUserNameSameAsOwner     = "SMB user name cannot be the same as the current account."
	ErrorMessageSmbPasswordLength          = "SMB password length must be between 6 and 16 characters."
	ErrorMessageSmbPasswordInvalid         = "SMB password may contain only printable ASCII characters and must not start or end with a space."
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

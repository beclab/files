package seahub

import (
	"os"
	"path"
	"strings"

	"files/pkg/models"

	"k8s.io/klog/v2"
)

type FileType int

const (
	TEXT FileType = iota
	MARKDOWN
	IMAGE
	VIDEO
	AUDIO
	PDF
	SVG
	XMIND
	DOCUMENT
	SPREADSHEET
	SEADOC
	UNKNOWN
)

var fileTypeNames = map[FileType]string{
	TEXT:        "TEXT",
	MARKDOWN:    "MARKDOWN",
	IMAGE:       "IMAGE",
	VIDEO:       "VIDEO",
	AUDIO:       "AUDIO",
	PDF:         "PDF",
	SVG:         "SVG",
	XMIND:       "XMIND",
	DOCUMENT:    "DOCUMENT",
	SPREADSHEET: "SPREADSHEET",
	SEADOC:      "SEADOC",
	UNKNOWN:     "UNKNOWN",
}

func FileTypeName(ft FileType) string {
	if name, exists := fileTypeNames[ft]; exists {
		return name
	}
	return "UNKNOWN"
}

var PREVIEW_FILE_EXT = map[FileType][]string{
	IMAGE:       {"gif", "jpeg", "jpg", "png", "ico", "bmp", "tif", "tiff", "psd", "avif", "webp", "heic"},
	DOCUMENT:    {"doc", "docx", "docxf", "oform", "ppt", "pptx", "odt", "fodt", "odp", "fodp"},
	SPREADSHEET: {"xls", "xlsx", "ods", "fods"},
	SVG:         {"svg"},
	PDF:         {"pdf", "ai"},
	MARKDOWN:    {"markdown", "md"},
	VIDEO:       {"mp4", "ogv", "webm", "mov", "avi", "wmv", "mkv", "flv", "rmvb", "rm", "3gp", "mpg", "vob"},
	AUDIO:       {"mp3", "oga", "ogg", "wav", "flac", "opus"},
	XMIND:       {"xmind"},
	SEADOC:      {"sdoc"},
}

var FILE_EXT_TYPE_MAP = generateFileExtTypeMap()

var confTextExt = []string{"txt", "log", "md"}

func generateFileExtTypeMap() map[string]FileType {
	fileExtTypeMap := make(map[string]FileType)
	for fileType, exts := range PREVIEW_FILE_EXT {
		for _, ext := range exts {
			fileExtTypeMap[ext] = fileType
		}
	}
	return fileExtTypeMap
}

func getFileTypeAndExt(filename string) (FileType, string) {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(filename), "."))

	if contains(confTextExt, ext) {
		return TEXT, ext
	}

	if fileType, exists := FILE_EXT_TYPE_MAP[ext]; exists {
		return fileType, ext
	}

	return UNKNOWN, ext
}

// SyncPermToMode maps a Seafile user_perm string to the local dir mode used
// when materializing a paste target. It routes through the unified
// LevelFromSyncPermission so preview/cloud-edit/admin (which the old
// rwx-string matching did not recognize, yielding an unusable mode 0) get a
// sensible readable/writable mode.
func SyncPermToMode(permStr string) os.FileMode {
	switch models.LevelFromSyncPermission(strings.TrimSpace(permStr)) {
	case models.LevelRead:
		return 0555
	case models.LevelWrite, models.LevelAdmin:
		return 0755
	default:
		klog.Infof("[sync] unrecognized permission string for mode: %q", permStr)
		return 0
	}
}

package net

/*

import (
	"fmt"
	"path/filepath"
	"strings"
)

var (
	videoFileExtensions = map[string]bool{
		".3gp":   true,
		".asf":   true,
		".avi":   true,
		".divx":  true,
		".dvr-ms": true,
		".f4v":   true,
		".flv":   true,
		".img":   true,
		".iso":   true,
		".m2t":   true,
		".m2ts":  true,
		".m2v":   true,
		".m4v":   true,
		".mk3d":  true,
		".mkv":   true,
		".mov":   true,
		".mp4":   true,
		".mpg":   true,
		".mpeg":  true,
		".mts":   true,
		".ogg":   true,
		".ogm":   true,
		".ogv":   true,
		".rec":   true,
		".ts":    true,
		".rmvb":  true,
		".webm":  true,
		".wmv":   true,
		".wtv":   true,
	}

	mimeTypeLookup = map[string]string{
		// Type application
		".azw3": "application/vnd.amazon.ebook",
		".cb7":  "application/x-cb7",
		".cba":  "application/x-cba",
		".cbr":  "application/vnd.comicbook-rar",
		".cbt":  "application/x-cbt",
		".cbz":  "application/vnd.comicbook+zip",

		// Type image
		".tbn": "image/jpeg",

		// Type text
		".ass": "text/x-ssa",
		".ssa": "text/x-ssa",
		".edl": "text/plain",
		".html": "text/html; charset=UTF-8",
		".htm":  "text/html; charset=UTF-8",

		// Type video
		".mpegts": "video/mp2t",

		// Type audio
		".aac":   "audio/aac",
		".ac3":   "audio/ac3",
		".ape":   "audio/x-ape",
		".dsf":   "audio/dsf",
		".dsp":   "audio/dsp",
		".flac":  "audio/flac",
		".m4b":   "audio/mp4",
		".mp3":   "audio/mpeg",
		".vorbis": "audio/vorbis",
		".webma": "audio/webm",
		".wv":    "audio/x-wavpack",
		".xsp":   "audio/xsp",
	}

	extensionLookup = map[string]string{
		// Type application
		"application/vnd.comicbook-rar": ".cbr",
		"application/vnd.comicbook+zip": ".cbz",
		"application/x-cb7":             ".cb7",
		"application/x-cba":             ".cba",
		"application/x-cbr":             ".cbr",
		"application/x-cbt":             ".cbt",
		"application/x-cbz":             ".cbz",
		"application/x-javascript":      ".js",
		"application/xml":               ".xml",
		"application/x-mpegURL":         ".m3u8",

		// Type audio
		"audio/aac":        ".aac",
		"audio/ac3":         ".ac3",
		"audio/dsf":         ".dsf",
		"audio/dsp":         ".dsp",
		"audio/flac":        ".flac",
		"audio/m4b":         ".m4b",
		"audio/vorbis":      ".vorbis",
		"audio/x-ape":       ".ape",
		"audio/xsp":         ".xsp",
		"audio/x-wavpack":   ".wv",

		// Type image
		"image/jpeg": ".jpg",
		"image/tiff": ".tiff",
		"image/x-png": ".png",
		"image/x-icon": ".ico",

		// Type text
		"text/plain": ".txt",
		"text/rtf":   ".rtf",
		"text/x-ssa": ".ssa",

		// Type video
		"video/vnd.mpeg.dash.mpd": ".mpd",
		"video/x-matroska":        ".mkv",
	}
)

func GetMimeType(filename string, defaultValue string) string {
	if filename == "" {
		return defaultValue
	}

	ext := filepath.Ext(filename)

	if mimeType, ok := mimeTypeLookup[ext]; ok {
		return mimeType
	}

	if mimeType, ok := Model.GetMimeType(filename); ok {
		return mimeType
	}

	// Catch-all for all video types that don't require specific mime types
	if _, ok := videoFileExtensions[ext]; ok {
		return "video/" + ext[1:]
	}

	return defaultValue
}

func ToExtension(mimeType string) string {
	if mimeType == "" {
		return ""
	}

	// handle text/html; charset=UTF-8
	mimeType = strings.Split(mimeType, ";")[0]

	if ext, ok := extensionLookup[mimeType]; ok {
		return ext
	}

	ext, _ := Model.GetMimeTypeExtension(mimeType)
	if ext != "" {
		return "." + ext
	}

	return ""
}

func IsImage(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}

type Model struct{}

func (Model) GetMimeType(filename string) (string, bool) {
	// Implement your own logic to get the MIME type from the filename
	// This is a placeholder implementation
	return "", false
}

func (Model) GetMimeTypeExtension(mimeType string) (string, bool) {
	// Implement your own logic to get the file extension from the MIME type
	// This is a placeholder implementation
	return "", false
}
*/

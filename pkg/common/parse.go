package common

import (
	"encoding/base64"
	"path"
	"strconv"
	"strings"
	"time"
)

func ParseBool(s string) bool {
	return s == "true" || s == "1"
}

func ParseInt(s string) (int, error) {
	r, err := strconv.Atoi(s)
	if err != nil {
		return r, err
	}
	return r, nil

}

func ParseInt64(s string) (int64, error) {
	r, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return r, err
	}
	return r, nil
}

func ParseUnixMilli(s int64) time.Time {
	var date = time.UnixMilli(s)

	return date
}

func MimeTypeByExtension(filename string) string {
	ext := strings.ToLower(path.Ext(filename))
	switch ext {
	case ".doc":
		return "application/msword"
	case ".docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case ".odt":
		return "application/vnd.oasis.opendocument.text"
	case ".pages":
		return "application/vnd.apple.pages"
	case ".pdf":
		return "application/pdf"
	case ".ppt", ".pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	case ".xls", ".xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case ".rtf":
		return "application/rtf"
	case ".xml":
		return "text/xml"
	case ".xhtml", ".html", ".htm":
		return "text/html"
	case ".jpg", ".jpeg", ".jpe", ".jfif", ".jfif-tbnl":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".tif", ".tiff":
		return "image/tiff"
	case ".txt":
		return "text/plain"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".zip":
		return "application/zip"
	case ".rar":
		return "application/x-rar-compressed"
	case ".7z":
		return "application/x-7z-compressed"
	case ".tar":
		return "application/x-tar"
	case ".gz":
		return "application/gzip"
	case ".bz2":
		return "application/x-bzip2"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".aac":
		return "audio/aac"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".avi":
		return "video/x-msvideo"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	default:
		return "application/octet-stream"
	}
}

func AdjustExpire(expireIn int64, expireTime string) (int64, string) {
	currentTime := time.Now()
	currentTimeMillis := currentTime.UnixMilli()

	if expireTime == "" {
		if expireIn == 0 {
			return 0, "9999-12-31T23:59:59.000000Z"
		}
		return expireIn, currentTime.Add(time.Duration(expireIn) * time.Millisecond).Format(time.RFC3339)
	}

	var expireTimeMillis int64
	if _, err := strconv.ParseInt(expireTime, 10, 64); err == nil {
		val, _ := strconv.ParseInt(expireTime, 10, 64)
		expireTimeMillis = val
	} else {
		parsedTime, err := time.Parse(time.RFC3339, expireTime)
		if err != nil {
			return 0, ""
		}
		expireTimeMillis = parsedTime.UnixMilli()
	}

	if expireIn != 0 {
		calculatedExpireTimeMillis := currentTimeMillis + expireIn
		if calculatedExpireTimeMillis > expireTimeMillis {
			return expireIn, currentTime.Add(time.Duration(expireIn) * time.Millisecond).Format(time.RFC3339)
		}
		return expireTimeMillis - currentTimeMillis, time.Unix(0, expireTimeMillis*int64(time.Millisecond)).Format(time.RFC3339)
	}

	remaining := expireTimeMillis - currentTimeMillis
	if remaining < 0 {
		return 0, time.Unix(0, expireTimeMillis*int64(time.Millisecond)).Format(time.RFC3339)
	}
	return remaining, time.Unix(0, expireTimeMillis*int64(time.Millisecond)).Format(time.RFC3339)
}

func Base64Encode(v string) string {
	return base64.StdEncoding.EncodeToString([]byte(v))
}

func Base64Decode(v string) (string, error) {
	r, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return "", err
	}

	return string(r), nil
}

package parser

import (
	"bufio"
	"code.sajari.com/docconv"
	"os"
	"path"
	"strings"
)

var ParseAble = map[string]bool{
	".doc":      true,
	".docx":     true,
	".pdf":      true,
	".txt":      true,
	".md":       true,
	".markdown": true,
}

func IsParseAble(filename string) bool {
	fileType := GetTypeFromName(filename)
	_, ok := ParseAble[fileType]
	return ok
}

func GetTypeFromName(filename string) string {
	return strings.ToLower(path.Ext(filename))
}

func ParseDoc(filepath string) (string, error) {
	fileType := GetTypeFromName(filepath)
	if _, ok := ParseAble[fileType]; !ok {
		return "", nil
	}

	var result strings.Builder

	if fileType == ".txt" || fileType == ".md" || fileType == ".markdown" {
		file, err := os.Open(filepath)
		if err != nil {
			return "", err
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			result.WriteString(scanner.Text())
			result.WriteString("\n")
		}

		if err := scanner.Err(); err != nil {
			return "", err
		}

		return result.String(), nil
	}

	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	mimeType := MimeTypeByExtension(filepath)
	res, err := docconv.Convert(file, mimeType, true)
	if err != nil {
		return "", err
	}
	return res.Body, nil
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

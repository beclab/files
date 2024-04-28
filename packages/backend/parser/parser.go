package parser

import (
	"io"
	"io/ioutil"
	"path"
	"strings"

	"code.sajari.com/docconv"
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

func ParseDoc(f io.Reader, filename string) (string, error) {
	fileType := GetTypeFromName(filename)
	if _, ok := ParseAble[fileType]; !ok {
		return "", nil
	}
	if fileType == ".txt" || fileType == ".md" || fileType == ".markdown" {
		data, err := ioutil.ReadAll(f)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	mimeType := docconv.MimeTypeByExtension(filename)
	res, err := docconv.Convert(f, mimeType, true)
	if err != nil {
		return "", err
	}
	return res.Body, nil
}

package encoder

import (
	"fmt"
	"strings"

	"files/pkg/media/mediabrowser/model/mediainfo/mediaprotocol"
)

func GetInputArgument(inputPrefix string, inputFile string, protocol mediaprotocol.MediaProtocol) string {
	if protocol != mediaprotocol.File {
		return fmt.Sprintf("\"%s\"", inputFile)
	}
	return GetFileInputArgument(inputFile, inputPrefix)
}

func GetInputArgumentArray(inputPrefix string, inputFiles []string, protocol mediaprotocol.MediaProtocol) string {
	if protocol != mediaprotocol.File {
		return fmt.Sprintf("\"%s\"", inputFiles[0])
	}
	return GetConcatInputArgument(inputFiles, inputPrefix)
}

func GetConcatInputArgument(inputFiles []string, inputPrefix string) string {
	if len(inputFiles) > 1 {
		files := make([]string, len(inputFiles))
		for i, file := range inputFiles {
			files[i] = NormalizePath(file)
		}
		return fmt.Sprintf("concat:\"%s\"", strings.Join(files, "|"))
	}
	return GetFileInputArgument(inputFiles[0], inputPrefix)
}

func GetFileInputArgument(path, inputPrefix string) string {
	if strings.Contains(path, "://") {
		return fmt.Sprintf("\"%s\"", path)
	}
	path = NormalizePath(path)
	return fmt.Sprintf("%s:\"%s\"", inputPrefix, path)
}

func NormalizePath(path string) string {
	formattedPath := path
	doubleQuoteSpecialChars := []string{
		"$",  // Variable/command substitution
		"`",  // Command substitution (backtick)
		"\"", // Double quote
	}

	// Escape double-quote special characters
	for _, char := range doubleQuoteSpecialChars {
		formattedPath = strings.ReplaceAll(formattedPath, char, "\\"+char)
	}

	return formattedPath
}

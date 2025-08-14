package common

import (
	"fmt"
	"strings"

	"github.com/spf13/afero"
)

func CollectDupNames(p string, prefixName string, ext string, isDir bool) ([]string, error) {
	// p = strings.Split(p,"/")[:len(x)-2]
	var result []string
	var afs = afero.NewOsFs()
	entries, err := afero.ReadDir(afs, p)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		// if entry.IsDir() != isDir {
		// 	continue
		// }

		infoName := entry.Name()
		if isDir {
			if strings.Contains(infoName, prefixName) {
				result = append(result, infoName)
			}
		} else {
			infoName = strings.TrimSuffix(infoName, ext)
			if strings.Contains(infoName, prefixName) {
				result = append(result, infoName)
			}
		}
	}

	return result, nil
}

func GenerateDupCommonName(existsName []string, prefixName string, existSamePathName string) string {
	if existSamePathName == "" {
		return prefixName
	}
	var filePrefixName = prefixName

	var count = 0
	var matchedCount int

	var searchName = prefixName

	for {
		var find bool
		for _, name := range existsName {
			if strings.TrimSpace(name) == searchName {
				find = true
				break
			}
		}

		if find {
			count++
			searchName = fmt.Sprintf("%s(%d)", prefixName, count)
			continue
		} else {
			matchedCount = count
			break
		}

	}

	var newFileName string
	if matchedCount == 0 {
		newFileName = filePrefixName
	} else {
		newFileName = fmt.Sprintf("%s(%d)", filePrefixName, matchedCount)
	}

	return newFileName
}

func GetFileNameFromPath(s string) (string, bool) {

	var isFile = strings.HasSuffix(s, "/")
	var tmp = strings.TrimSuffix(s, "/")
	var p = strings.LastIndex(tmp, "/")
	var r = tmp[p:]
	r = strings.Trim(r, "/")

	return r, !isFile
}

func GetPrefixPath(s string) string {
	// /a/b/hello.txt   > /a/b/
	// /a/b/c/          > /a/b/
	if s == "/" {
		return s
	}

	var r = strings.TrimSuffix(s, "/")
	var p = strings.LastIndex(r, "/")
	return r[:p+1]
}

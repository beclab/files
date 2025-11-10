package samba

import (
	"fmt"
	"sort"
	"strings"
)

func GetSambaShareName(sharePath string) (string, error) {
	var tmp = strings.Trim(sharePath, "/")
	var s = strings.Split(tmp, "/")
	if len(s) == 0 {
		return "", fmt.Errorf("smb share path %s invalid", sharePath)
	}

	return s[len(s)-1], nil
}

func GetSambaShareDupName(smbShareName string, sharePaths []string) (string, error) {
	var err error
	var tmp string
	var lastNames []string
	for _, s := range sharePaths {
		tmp, err = GetSambaShareName(s)
		if err != nil {
			break
		}
		lastNames = append(lastNames, tmp)
	}

	if err != nil {
		return "", err
	}

	sort.Strings(lastNames)

	var count = 0
	var matchedCount = 0
	var searchName = smbShareName

	for {
		var find bool
		for _, name := range lastNames {
			if name == searchName {
				find = true
				break
			}
		}

		if find {
			count++
			searchName = fmt.Sprintf("%s%d", smbShareName, count)
			continue
		} else {
			matchedCount = count
			break
		}
	}

	var newSmbShareName string
	if matchedCount == 0 {
		newSmbShareName = smbShareName
	} else {
		newSmbShareName = fmt.Sprintf("%s%d", smbShareName, matchedCount)
	}

	return newSmbShareName, nil
}

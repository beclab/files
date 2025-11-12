package samba

import (
	"files/pkg/hertz/biz/model/api/share"
	"files/pkg/models"
	"fmt"
	"sort"
	"strings"
)

func FormatSharePathViews(data []*share.SmbShareView) map[string]*models.SambaShares {
	if len(data) == 0 {
		return nil
	}

	sort.Slice(data, func(i, j int) bool {
		return data[i].ID < data[j].ID
	})

	var result = make(map[string]*models.SambaShares)

	for _, d := range data {
		val, ok := result[d.ID]
		if !ok {
			val = &models.SambaShares{
				Id:          d.ID,
				Owner:       d.Owner,
				FileType:    d.FileType,
				Extend:      d.Extend,
				Path:        d.Path,
				ShareType:   d.ShareType,
				Name:        d.Name,
				ExpireIn:    d.ExpireIn,
				ExpireTime:  d.ExpireTime,
				Permission:  d.SharePermission,
				PublicShare: d.SmbSharePublic == 1,
				Members:     make([]*models.SambaShareMembers, 0),
			}
			if d.UserName != "" {
				var member = &models.SambaShareMembers{
					UserId:     d.UserId,
					UserName:   d.UserName,
					Password:   d.Password,
					Permission: d.Permission,
				}
				val.Members = append(val.Members, member)
			}
			result[d.ID] = val
			continue
		}

		if d.UserName != "" {
			var member = &models.SambaShareMembers{
				UserId:     d.UserId,
				UserName:   d.UserName,
				Password:   d.Password,
				Permission: d.Permission,
			}
			val.Members = append(val.Members, member)
		}
		result[d.ID] = val
	}

	return result
}

func CompareSmbShareMembers(existsMembers []*share.ShareSmbMember, modifyMembers []*share.CreateSmbSharePathMembers) (newMembers, editMembers, delMembers []*share.CreateSmbSharePathMembers) {
	existsIdx := make(map[string]*share.ShareSmbMember, len(existsMembers))
	for _, n := range existsMembers {
		existsIdx[n.UserID] = n
	}
	modIdx := make(map[string]*share.CreateSmbSharePathMembers, len(modifyMembers))
	for _, m := range modifyMembers {
		modIdx[m.ID] = m
	}

	for id, m := range modIdx {
		if old, ok := existsIdx[id]; !ok {
			newMembers = append(newMembers, m)
		} else if m.Permission != old.Permission {
			editMembers = append(editMembers, m)
		}
	}

	for id, old := range existsIdx {
		if _, ok := modIdx[id]; !ok {
			delMembers = append(delMembers, &share.CreateSmbSharePathMembers{
				ID:         id,
				Permission: old.Permission,
			})
		}
	}

	return
}

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

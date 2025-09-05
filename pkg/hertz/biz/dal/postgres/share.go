package postgres

import (
	"files/pkg/common"
	"files/pkg/hertz/biz/model/share"
	"fmt"
	"k8s.io/klog/v2"
	"strconv"
	"time"
)

// share_path

func CreateSharePath(paths []*share.SharePath) ([]*share.SharePath, error) {
	result := DB.Create(paths)
	if result.Error != nil {
		return nil, result.Error
	}

	return paths, nil
}

func DeleteSharePath(pathID string) error {
	return DB.Where("id = ?", pathID).Delete(&share.SharePath{}).Error
}

func UpdateSharePath(path *share.SharePath) error {
	return DB.Model(path).
		Select("*").
		Omit("CreateTime", "UpdateTime").
		Where("id = ?", path.ID).
		Updates(path).Error
}

func QuerySharePath(params *QueryParams, page, pageSize int64, orderBy, order string, joinParams []*JoinCondition) ([]*share.SharePath, int64, error) {
	var res []*share.SharePath
	total, err := QueryData(&share.SharePath{}, &res, params, page, pageSize, orderBy, order, joinParams)
	if err != nil {
		klog.Error(err)
		return nil, 0, err
	}
	return res, total, nil
}

func UpdateSharePathFields(pathID string, updates map[string]interface{}) error {
	return DB.Model(&share.SharePath{}).
		Where("id = ?", pathID).
		Updates(updates).Error
}

func VerifySharePathPassword(pathID, inputPassword string) (bool, error) {
	var path share.SharePath
	if err := DB.Select("password_md5").Where("id = ?", pathID).First(&path).Error; err != nil {
		return false, err
	}
	return path.PasswordMd5 == common.Md5String(inputPassword), nil
}

func CheckSharePathExpired(pathID string) (bool, error) {
	var path share.SharePath
	if err := DB.Select("expire_time").Where("id = ?", pathID).First(&path).Error; err != nil {
		return false, err
	}

	if path.ExpireTime == "" {
		return false, nil
	}

	var expireTime time.Time
	if _, err := strconv.ParseInt(path.ExpireTime, 10, 64); err == nil {
		millis, _ := strconv.ParseInt(path.ExpireTime, 10, 64)
		expireTime = time.UnixMilli(millis).UTC()
	} else {
		var err error
		expireTime, err = time.Parse(time.RFC3339, path.ExpireTime)
		if err != nil {
			return false, fmt.Errorf("invalid expire_time format: %s", path.ExpireTime)
		}
	}

	now := time.Now().UTC()
	return now.After(expireTime), nil
}

// share_token

func CreateShareToken(tokens []*share.ShareToken) ([]*share.ShareToken, error) {
	result := DB.Create(tokens)
	if result.Error != nil {
		return nil, result.Error
	}

	return tokens, nil
}

func DeleteShareToken(token string) error {
	return DB.Where("token = ?", token).Delete(&share.ShareToken{}).Error
}

func UpdateShareToken(token *share.ShareToken) error {
	return DB.Model(token).
		Select("*").
		Where("id = ?", token.ID).
		Updates(token).Error
}

func QueryShareToken(params *QueryParams, page, pageSize int64, orderBy, order string, joinParams []*JoinCondition) ([]*share.ShareToken, int64, error) {
	var res []*share.ShareToken
	total, err := QueryData(&share.ShareToken{}, &res, params, page, pageSize, orderBy, order, joinParams)
	if err != nil {
		klog.Error(err)
		return nil, 0, err
	}
	return res, total, nil
}

func UpdateShareTokenFields(tokenID int64, updates map[string]interface{}) error {
	return DB.Model(&share.ShareToken{}).
		Where("id = ?", tokenID).
		Updates(updates).Error
}

// share member

func CreateShareMember(members []*share.ShareMember) ([]*share.ShareMember, error) {
	result := DB.Create(members)
	if result.Error != nil {
		return nil, result.Error
	}

	return members, nil
}

func DeleteShareMember(memberID int64) error {
	return DB.Where("id = ?", memberID).Delete(&share.ShareMember{}).Error
}

func UpdateShareMember(member *share.ShareMember) error {
	return DB.Model(member).
		Select("*").
		Omit("CreateTime", "UpdateTime").
		Where("id = ?", member.ID).
		Updates(member).Error
}

func QueryShareMember(params *QueryParams, page, pageSize int64, orderBy, order string, joinParams []*JoinCondition) ([]*share.ShareMember, int64, error) {
	var res []*share.ShareMember
	total, err := QueryData(&share.ShareMember{}, &res, params, page, pageSize, orderBy, order, joinParams)
	if err != nil {
		klog.Error(err)
		return nil, 0, err
	}
	return res, total, nil
}

func UpdateShareMemberFields(memberID int64, updates map[string]interface{}) error {
	return DB.Model(&share.ShareMember{}).
		Where("id = ?", memberID).
		Updates(updates).Error
}

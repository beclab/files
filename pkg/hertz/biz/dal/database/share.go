package database

import (
	"files/pkg/common"
	"files/pkg/hertz/biz/model/api/share"
	"fmt"
	"gorm.io/gorm"
	"strconv"
	"time"

	"k8s.io/klog/v2"
)

// share_path

func CreateSharePath(paths []*share.SharePath, db *gorm.DB) ([]*share.SharePath, error) {
	result := db.Create(paths)
	if result.Error != nil {
		return nil, result.Error
	}

	return paths, nil
}

func DeleteSharePath(pathID string, db *gorm.DB) error {
	return db.Where("id = ?", pathID).Delete(&share.SharePath{}).Error
}

func UpdateSharePath(pathID string, updates map[string]interface{}, db *gorm.DB) error {
	if updates["update_time"] == nil {
		updates["update_time"] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return db.Model(&share.SharePath{}).
		Where("id = ?", pathID).
		Updates(updates).Error
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

func CreateShareToken(tokens []*share.ShareToken, db *gorm.DB) ([]*share.ShareToken, error) {
	result := db.Create(tokens)
	if result.Error != nil {
		return nil, result.Error
	}

	return tokens, nil
}

func DeleteShareToken(token string, db *gorm.DB) error {
	return db.Where("token = ?", token).Delete(&share.ShareToken{}).Error
}

func UpdateShareToken(tokenID int64, updates map[string]interface{}, db *gorm.DB) error {
	return db.Model(&share.ShareToken{}).
		Where("id = ?", tokenID).
		Updates(updates).Error
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

// share member

func CreateShareMember(members []*share.ShareMember, db *gorm.DB) ([]*share.ShareMember, error) {
	result := db.Create(members)
	if result.Error != nil {
		return nil, result.Error
	}

	return members, nil
}

func DeleteShareMember(memberID int64, db *gorm.DB) error {
	return db.Where("id = ?", memberID).Delete(&share.ShareMember{}).Error
}

func UpdateShareMember(memberID int64, updates map[string]interface{}, db *gorm.DB) error {
	if updates["update_time"] == nil {
		updates["update_time"] = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return db.Model(&share.ShareMember{}).
		Where("id = ?", memberID).
		Updates(updates).Error
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

func QueryShareById(shareId string) (*share.SharePath, error) {
	var res *share.SharePath
	if err := DB.Table("share_paths").Where("id = ?", shareId).First(&res).Error; err != nil {
		return nil, err
	}

	return res, nil
}

func QueryShareMemberById(shareId string) (*share.ShareMember, error) {
	var res *share.ShareMember
	if err := DB.Table("share_members").Where("path_id = ?", shareId).First(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func QueryShareExternalById(shareId string) (*share.ShareToken, error) {
	var res *share.ShareToken
	if err := DB.Table("share_tokens").Where("path_id = ?", shareId).First(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

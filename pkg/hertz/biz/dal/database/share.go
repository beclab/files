package database

import (
	"files/pkg/common"
	"files/pkg/hertz/biz/model/api/share"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"

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

func QuerySharePathByType(shareType string) ([]*share.SharePath, error) {
	var res []*share.SharePath
	if err := DB.Where("share_type = ?", shareType).Find(&res).Error; err != nil {
		return nil, err
	}

	return res, nil
}

func QuerySmbShares(shareType string) ([]*share.SmbShareView, error) {
	// SELECT share_paths.id, share_paths.owner, share_paths.file_type, share_paths.extend, share_paths.path, share_paths.share_type, share_paths.name, share_paths.expire_in, share_paths.expire_time, share_paths.smb_share_public, share_smb_members.permission, share_smb_users.user_id,  share_smb_users.user_name, share_smb_users.password FROM "share_paths" inner join share_smb_members on share_paths.id = share_smb_members.path_id inner join share_smb_users on share_smb_members.user_id = share_smb_users.user_id WHERE share_paths.share_type = 'smb'
	var res []*share.SmbShareView
	err := DB.Table("share_paths").Select("share_paths.id, share_paths.owner, share_paths.file_type, share_paths.extend, share_paths.path, share_paths.share_type, share_paths.name, share_paths.expire_in, share_paths.expire_time, share_paths.smb_share_public, share_smb_members.permission, share_smb_users.user_id,  share_smb_users.user_name, share_smb_users.password").Joins("inner join share_smb_members on share_paths.id = share_smb_members.path_id").Joins("inner join share_smb_users on share_smb_members.user_id = share_smb_users.user_id").Where("share_paths.share_type = ?", shareType).Scan(&res).Error

	if err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, nil
	}

	return res, nil
}

func QuerySmbSharePathByIds(ids []string) ([]*share.SharePath, error) {
	var res []*share.SharePath
	if err := DB.Where("share_type = ? and id in ?", "smb", ids).Find(&res).Error; err != nil {
		return nil, err
	}

	return res, nil
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

func GetSharePath(pathID string) (*share.SharePath, error) {
	var res *share.SharePath
	if err := DB.Table("share_paths").Where("id = ?", pathID).First(&res).Error; err != nil {
		return nil, err
	}

	return res, nil
}

// share_token

func CreateShareToken(tokens []*share.ShareToken, db *gorm.DB) ([]*share.ShareToken, error) {
	result := db.Create(tokens)
	if result.Error != nil {
		return nil, result.Error
	}

	return tokens, nil
}

func GetShareToken(token string) (*share.ShareToken, error) {
	var res *share.ShareToken
	if err := DB.Table("share_tokens").Where("token = ?", token).First(&res).Error; err != nil {
		return nil, err
	}

	return res, nil
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
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, nil
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

func QueryShareExternalById(shareId string, token string) (*share.ShareToken, error) {
	var res *share.ShareToken
	if err := DB.Table("share_tokens").Where("path_id = ? and token = ?", shareId, token).First(&res).Error; err != nil {
		return nil, err
	}
	return res, nil
}

func QuerySearchSharedDirectories(owner string) ([]*share.SharePath, []*share.SharePath, error) {
	var selectFields = "id,owner,file_type,extend,path,share_type,name,permission"

	// ~ internal
	var internalSharePaths []*share.SharePath
	var internalSub = DB.Model(&share.ShareMember{})
	internalSub.Select("path_id").Where("share_member LIKE ?", fmt.Sprintf("%%%s%%", owner))

	var db = DB.Model(&share.SharePath{})
	if err := db.Select(selectFields).Where("share_type = ? AND (owner = ? OR id IN (?)) AND expire_time > now()", "internal", owner, internalSub).Find(&internalSharePaths).Error; err != nil {
		return nil, nil, err
	}

	// ~ external
	var externalSharePaths []*share.SharePath
	var permissions = []int{1, 2, 3, 4}
	var externalSub = DB.Model(&share.ShareToken{})

	externalSub.Select("path_id").Where("expire_at > now()")

	db = DB.Model(&share.SharePath{})
	if err := db.Select(selectFields).Where("share_type = ? AND permission IN ? AND expire_time > now() AND id IN (?)", "external", permissions, externalSub).Find(&externalSharePaths).Error; err != nil {
		return nil, nil, err
	}

	return internalSharePaths, externalSharePaths, nil

}

func UpdateSharePassword(sharePath *share.SharePath) error {
	return DB.Table("share_paths").Save(sharePath).Error
}

func GetSmbSharePathByPath(shareType string, fileType, extend, path string, owner string, isPublic int32) (*share.SharePath, error) {
	var res *share.SharePath

	if err := DB.Table("share_paths").Where("share_type = ? AND owner = ? AND file_type = ? AND extend = ? AND path = ? AND smb_share_public = ?", shareType, owner, fileType, extend, path, isPublic).Scan(&res).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, err
	}
	return res, nil
}

func CreateSmbSharePathTx(sharePath *share.SharePath, shareMembers []*share.ShareSmbMember) error {
	var err error
	tx := DB.Begin()

	if err = tx.Create(sharePath).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, m := range shareMembers {
		if err = tx.Create(m).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	err = tx.Commit().Error
	if err != nil {
		if err = tx.Rollback().Error; err != nil {
			return err
		}
	}

	return nil
}

func QuerySmbUsers() ([]*share.ShareSmbUser, error) {
	var res []*share.ShareSmbUser
	if err := DB.Table("share_smb_users").Find(&res).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, nil
	}

	return res, nil
}

func CreateSmbUser(owner, userId, userName, password string) error {
	var now = time.Now().UTC().Format(time.RFC3339Nano)
	var data = &share.ShareSmbUser{
		Owner:      owner,
		UserID:     userId,
		UserName:   userName,
		Password:   password,
		CreateTime: now,
		UpdateTime: now,
	}

	return DB.Table("share_smb_users").Create(data).Error
}

func DeleteSmbUser(owner string, users []string) error {
	var err error
	var tx = DB.Begin()

	for _, user := range users {
		if err = tx.Table("share_smb_members").Where("owner = ? AND user_id = ?", owner, user).Delete(&share.ShareSmbMember{}).Error; err != nil {
			tx.Rollback()
			return err
		}

		if err = tx.Table("share_smb_users").Where("owner = ? AND user_id = ?", owner, user).Delete(&share.ShareSmbUser{}).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	err = tx.Commit().Error
	if err != nil {
		if err = tx.Rollback().Error; err != nil {
			return err
		}
	}

	return nil
}

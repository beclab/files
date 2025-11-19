package database

import (
	"files/pkg/common"
	"files/pkg/hertz/biz/model/api/share"
	"files/pkg/models"
	"fmt"
	"os"
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

func QueryShareInternalMembers(shareIds []string) ([]*share.ShareMember, error) {
	var res []*share.ShareMember
	if err := DB.Table("share_members").Where("path_id IN ?", shareIds).Find(&res).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, nil
	}
	return res, nil
}

func QueryShareSmbMembers(shareIds []string) ([]*models.SambaMemberUserView, error) {
	var res []*models.SambaMemberUserView
	if err := DB.Table("share_smb_members").Select("share_smb_members.path_id, share_smb_users.user_id, share_smb_users.user_name, share_smb_members.permission").Joins("INNER JOIN share_smb_users ON share_smb_members.user_id = share_smb_users.user_id").Where("path_id IN ?", shareIds).Find(&res).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, nil
	}
	return res, nil
}

func QuerySmbShares(owner string, shareType string, shareIds []string, userIds []string) ([]*share.SmbShareView, error) {
	var res []*share.SmbShareView
	var tx = DB.Table("share_paths").Select("share_paths.id, share_paths.owner, share_paths.file_type, share_paths.extend, share_paths.path, share_paths.share_type, share_paths.name, share_paths.expire_in, share_paths.expire_time, share_paths.permission as share_permission, share_paths.smb_share_public, share_smb_members.permission, share_smb_users.user_id, share_smb_users.user_name, share_smb_users.password").Joins("LEFT JOIN share_smb_members ON share_paths.id = share_smb_members.path_id").Joins("LEFT JOIN share_smb_users ON share_smb_members.user_id = share_smb_users.user_id").Where("share_paths.share_type  = ?", shareType)

	if owner != "" {
		tx.Where("share_paths.owner = ?", owner)
	}
	if len(shareIds) > 0 {
		tx.Where("share_paths.id IN ?", shareIds)
	}
	if len(userIds) > 0 {
		tx.Where("share_smb_users.user_id IN ?", userIds)
	}

	err := tx.Scan(&res).Error

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
	if err := DB.Where("share_type = ? AND id IN ?", "smb", ids).Find(&res).Error; err != nil {
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

func GetSharePath(shareId string) (*share.SharePath, error) {
	var res *share.SharePath
	if err := DB.Table("share_paths").Where("id = ?", shareId).First(&res).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, nil
	}

	return res, nil
}

func GetShareMember(shareId string, member string) (*share.ShareMember, error) {
	var res *share.ShareMember
	if err := DB.Table("share_members").Where("path_id = ? AND share_member = ?", shareId, member).First(&res).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, nil
	}

	return res, nil
}

func GetInternalSmbSharePath(owner string, shareType string, shareFileType, shareExtend, sharePath string) (res *models.InternalSmbSharePath, err error) {
	if shareType == common.ShareTypeInternal {
		var data []*models.InternalSharePath
		if err = DB.Table("share_paths").Select("share_paths.*, share_members.id as share_member_id, share_members.share_member, share_members.permission as share_member_permission").Joins("LEFT JOIN share_members on share_paths.id = share_members.path_id").Where("share_paths.owner = ? AND share_paths.share_type = ? AND share_paths.file_type = ? AND share_paths.extend = ? AND share_paths.path = ?", owner, shareType, shareFileType, shareExtend, sharePath).Find(&data).Error; err != nil {
			return
		}
		if len(data) > 0 {
			var item = data[0]
			res = &models.InternalSmbSharePath{
				Id:         item.Id,
				Owner:      item.Owner,
				FileType:   item.FileType,
				Extend:     item.Extend,
				Path:       item.Path,
				ShareType:  item.ShareType,
				Name:       item.Name,
				ExpireIn:   item.ExpireIn,
				ExpireTime: item.ExpireTime,
				Permission: item.Permission,
				SharedByMe: item.Owner == owner,
				Users:      make([]*models.InternalSmbSharePathUsers, 0),
			}

			for _, d := range data {
				if d.ShareMember != "" {
					var user = &models.InternalSmbSharePathUsers{
						Id:         fmt.Sprintf("%d", d.ShareMemberId),
						Name:       d.ShareMember,
						Permission: d.ShareMemberPermission,
					}
					res.Users = append(res.Users, user)
				}
			}
		}
	} else {
		var data []*share.SmbShareView
		if err = DB.Table("share_paths").Select("share_paths.id, share_paths.owner, share_paths.file_type, share_paths.extend, share_paths.path, share_paths.share_type, share_paths.name, share_paths.expire_in, share_paths.expire_time, share_paths.permission as share_permission, share_paths.smb_share_public, share_smb_members.permission, share_smb_users.user_id, share_smb_users.user_name, share_smb_users.password").Joins("LEFT JOIN share_smb_members ON share_paths.id = share_smb_members.path_id").Joins("LEFT JOIN share_smb_users ON share_smb_members.user_id = share_smb_users.user_id").Where("share_paths.owner = ? AND share_paths.share_type = ? AND share_paths.file_type = ? AND share_paths.extend = ? AND share_paths.path = ?", owner, shareType, shareFileType, shareExtend, sharePath).Find(&data).Error; err != nil {
			return
		}

		if len(data) > 0 {
			var item = data[0]
			var publicSmb bool = (item.SmbSharePublic == 1)
			res = &models.InternalSmbSharePath{
				Id:         item.ID,
				Owner:      item.Owner,
				FileType:   item.FileType,
				Extend:     item.Extend,
				Path:       item.Path,
				ShareType:  item.ShareType,
				Name:       item.Name,
				ExpireIn:   item.ExpireIn,
				ExpireTime: item.ExpireTime,
				Permission: item.SharePermission,
				SharedByMe: item.Owner == owner,
				PublicSmb:  &publicSmb,
				Users:      make([]*models.InternalSmbSharePathUsers, 0),
				SmbLink:    fmt.Sprintf("smb://%s/%s", os.Getenv("NODE_IP"), item.Name),
			}

			for _, d := range data {
				if d.UserId != "" {
					var user = &models.InternalSmbSharePathUsers{
						Id:         d.UserId,
						Name:       d.UserName,
						Permission: d.Permission,
					}
					res.Users = append(res.Users, user)
				}
			}
		}
	}

	return
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

func QueryShareMembers(shareId string) ([]*share.ShareMember, error) {
	DB.Table("share_members").Where("path_id = ?", shareId)
	return nil, nil
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

func DeleteShareSmbMember(memberID int64, db *gorm.DB) error {
	return db.Where("id = ?", memberID).Delete(&share.ShareSmbMember{}).Error
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

func QueryShareSmbMember(params *QueryParams, page, pageSize int64, orderBy, order string, joinParams []*JoinCondition) ([]*share.ShareSmbMember, int64, error) {
	var res []*share.ShareSmbMember
	total, err := QueryData(&share.ShareSmbMember{}, &res, params, page, pageSize, orderBy, order, joinParams)
	if err != nil {
		klog.Error(err)
		return nil, 0, err
	}
	return res, total, nil
}

func QueryShareSmbUser(params *QueryParams, page, pageSize int64, orderBy, order string, joinParams []*JoinCondition) ([]*share.ShareSmbUser, int64, error) {
	var res []*share.ShareSmbUser
	total, err := QueryData(&share.ShareSmbUser{}, &res, params, page, pageSize, orderBy, order, joinParams)
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

func QueryShareMemberById(member string, shareId string) (*share.ShareMember, error) {
	var res *share.ShareMember
	if err := DB.Table("share_members").Where("path_id = ? AND share_member = ?", shareId, member).First(&res).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, nil
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

func QuerySearchSharedDirectories(owner string) ([]*share.ShareMember, []*share.SharePath, error) {
	// searched directories
	var selectFields = "id,owner,file_type,extend,path,share_type,name,permission"

	// ~ internal
	var internalSharePaths []*share.ShareMember
	if err := DB.Table("share_paths").Select("share_members.*").Joins("INNER JOIN share_members ON share_paths.id = share_members.path_id").Where("share_paths.share_type = ? AND share_members.share_member = ? AND share_paths.expire_time > now()", common.ShareTypeInternal, owner).Find(&internalSharePaths).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, nil, err
		}
	}

	// ~ external
	var externalSharePaths []*share.SharePath
	var permissions = []int{1, 2, 3, 4}
	var externalSub = DB.Model(&share.ShareToken{})
	externalSub.Select("path_id").Where("expire_at > now()")

	if err := DB.Table("share_paths").Select(selectFields).Where("share_type = ? AND permission IN ? AND expire_time > now() AND id IN (?)", "external", permissions, externalSub).Find(&externalSharePaths).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, nil, err
		}
	}
	return internalSharePaths, externalSharePaths, nil
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

func GetSmbSharePathByPrefix(fileType, extend, path string, owner string) ([]*share.SharePath, error) {
	var res []*share.SharePath

	if err := DB.Table("share_paths").Where("owner = ? AND file_type = ? AND extend = ? AND path LIKE ?", owner, fileType, extend, path+"%").Find(&res).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, err
	}
	return res, nil
}

func ResetExternalSharePasswordTx(sharePath *share.SharePath) error {
	var err error
	var tx = DB.Begin()

	var updates = make(map[string]interface{})
	updates["expire_at"] = time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339Nano)

	if err = tx.Table("share_paths").Where("id = ?", sharePath.ID).Update("password_md5", sharePath.PasswordMd5).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err = tx.Table("share_tokens").Where("path_id = ?", sharePath.ID).Updates(updates).Error; err != nil {
		tx.Rollback()
		return err
	}

	err = tx.Commit().Error
	if err != nil {
		if err = tx.Rollback().Error; err != nil {
			return err
		}
	}

	return nil
}

func CreateSmbSharePathTx(sharePath *share.SharePath, shareMembers []*share.ShareSmbMember) error {
	var err error
	tx := DB.Begin()

	if err = tx.Create(sharePath).Error; err != nil {
		tx.Rollback()
		return err
	}

	if len(shareMembers) > 0 {
		for _, m := range shareMembers {
			if err = tx.Create(m).Error; err != nil {
				tx.Rollback()
				return err
			}
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

func QuerySmbMembers(pathID string) ([]*share.ShareSmbMember, error) {
	var res []*share.ShareSmbMember
	if err := DB.Table("share_smb_members").Where("path_id = ?", pathID).Find(&res).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil, err
		}
		return nil, nil
	}

	return res, nil
}

func QuerySmbUsers(userIds []string) ([]*share.ShareSmbUser, error) {
	var res []*share.ShareSmbUser
	var tx = DB.Table("share_smb_users")
	if len(userIds) > 0 {
		tx.Where("user_id IN ?", userIds)
	}
	if err := tx.Find(&res).Error; err != nil {
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

func DeleteSmbUserTx(owner string, users []string) error {
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

func ModifySmbMembersTx(owner string, publicSmb bool, pathId string, addMembers, editMembers, delMembers []*share.CreateSmbSharePathMembers) error {
	var err error
	var tx = DB.Begin()
	var now = time.Now().UTC().Format(time.RFC3339Nano)

	var smbSharePublic int32 = 0
	if publicSmb {
		smbSharePublic = 1
	}

	if err = tx.Table("share_paths").Where("id = ?", pathId).Update("smb_share_public", smbSharePublic).Error; err != nil {
		tx.Rollback()
		return err
	}

	for _, member := range addMembers {
		var m = &share.ShareSmbMember{
			Owner:      owner,
			PathID:     pathId,
			UserID:     member.ID,
			Permission: member.Permission,
			CreateTime: now,
			UpdateTime: now,
		}
		if err = tx.Table("share_smb_members").Create(m).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	for _, member := range editMembers {
		var updates = make(map[string]interface{})
		updates["permission"] = member.Permission
		if err = tx.Table("share_smb_members").Where("owner = ? AND path_id = ? AND user_id = ?", owner, pathId, member.ID).Updates(updates).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	for _, member := range delMembers {
		if err = tx.Table("share_smb_members").Where("owner = ? AND path_id = ? AND user_id = ?", owner, pathId, member.ID).Delete(&share.ShareSmbMember{}).Error; err != nil {
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

func DeleteSmbShareTx(owner string, pathIds []string) error {
	var err error
	var tx = DB.Begin()

	if err = tx.Table("share_smb_members").Where("path_id IN ?", pathIds).Delete(&share.ShareSmbMember{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err = tx.Table("share_paths").Where("id IN ?", pathIds).Delete(&share.SharePath{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	err = tx.Commit().Error
	if err != nil {
		if err = tx.Rollback().Error; err != nil {
			return err
		}
	}
	return nil
}

func UpdateMovedChangeSharePathsEx(updateSharePaths map[string][5]string) error {
	var err error
	var tx = DB.Begin()

	for k, v := range updateSharePaths {
		var updates = make(map[string]interface{})
		var shareType = v[0]
		var smbOperator = v[1]
		updates["file_type"] = v[2]
		updates["extend"] = v[3]
		updates["path"] = v[4]
		if err = tx.Table("share_paths").Where("id = ?", k).Updates(updates).Error; err != nil {
			tx.Rollback()
			return err
		}

		if shareType == common.ShareTypeSMB {
			if smbOperator == "del" {
				if err = tx.Table("share_smb_members").Where("path_id = ?", k).Delete(&share.ShareSmbMember{}).Error; err != nil {
					tx.Rollback()
					return err
				}

				if err = tx.Table("share_paths").Where("id = ?", k).Delete(&share.SharePath{}).Error; err != nil {
					tx.Rollback()
					return err
				}
			}
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

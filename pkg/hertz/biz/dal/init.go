package dal

import (
	"files/pkg/hertz/biz/dal/database"
	"files/pkg/hertz/biz/model/api/share"
	"os"
	"strings"

	"k8s.io/klog/v2"
)

func migration(table interface{}, tableName string, rebuild bool) {
	if rebuild {
		if !database.DB.Migrator().HasTable(table) {
			klog.Infof("table %s does not exist, skipping rebuild", tableName)
			return
		}

		if err := database.DB.Migrator().DropTable(table); err != nil {
			if strings.Contains(err.Error(), "unknown table") ||
				strings.Contains(err.Error(), "does not exist") {
				klog.Infof("table %s already dropped", tableName)
			} else {
				klog.Errorf("failed to drop table %s: %v", tableName, err)
			}
			return
		}
	}

	err := database.DB.AutoMigrate(table)
	if err != nil {
		klog.Errorf("failed to migrate database %s: %v", tableName, err)
	} else {
		klog.Infof("migrated database table %s", tableName)
	}
}

func Init() {
	database.Init()

	rebuild := os.Getenv("REBUILD_DATABASE") == "true"
	migration(&share.SharePath{}, "share_paths", rebuild)
	migration(&share.ShareToken{}, "share_tokens", rebuild)
	migration(&share.ShareMember{}, "share_members", rebuild)
	migration(&share.ShareSmbUser{}, "share_smb_users", rebuild)
	migration(&share.ShareSmbMember{}, "share_smb_members", rebuild)

	cleanupOwnerAsShareMember()
}

// cleanupOwnerAsShareMember removes any share_members rows whose
// share_member equals the owner of their share_paths row. Such rows are
// phantom data caused by an old bug in UpdateSharePathMembers: when a
// non-owner admin (e.g. a sub-account granted admin) edited a share's
// members, the FE-submitted list often still carried the path's owner
// (used purely for display), and the handler would insert that owner
// into share_members. The phantom row makes the share appear twice in
// the owner's ListSharePath response — once as sharedByMe, once as
// sharedToMe via the share_members join.
//
// This cleanup is idempotent: on a clean DB it deletes 0 rows. The
// source of the bug is also fixed in the handler so no new phantom
// rows will be produced after upgrade; this exists to remediate
// historical data in place during a normal restart.
func cleanupOwnerAsShareMember() {
	// Sub-query: for each share_members row, "does a share_paths row
	// exist with the same id (= path_id) and owner == share_member?"
	exists := database.DB.Table("share_paths").
		Select("1").
		Where("share_paths.id = share_members.path_id").
		Where("share_paths.owner = share_members.share_member")

	res := database.DB.
		Where("EXISTS (?)", exists).
		Delete(&share.ShareMember{})
	if res.Error != nil {
		klog.Errorf("[share-cleanup] phantom share_members cleanup failed: %v", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		klog.Warningf("[share-cleanup] removed %d phantom share_members row(s) where share_member == sharePath.Owner", res.RowsAffected)
	} else {
		klog.Infof("[share-cleanup] no phantom share_members rows found")
	}
}

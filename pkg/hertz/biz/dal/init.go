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
}

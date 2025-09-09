package dal

import (
	"files/pkg/hertz/biz/dal/database"
	"files/pkg/hertz/biz/model/api/share"
	"k8s.io/klog/v2"
)

func migration(table interface{}, tableName string) {
	err := database.DB.AutoMigrate(table)
	if err != nil {
		klog.Errorf("failed to migrate database %s: %v", tableName, err)
	} else {
		klog.Infof("migrated database table %s", tableName)
	}
}

func Init() {
	database.Init()

	migration(&share.SharePath{}, "share_paths")
	migration(&share.ShareToken{}, "share_tokens")
	migration(&share.ShareMember{}, "share_members")
}

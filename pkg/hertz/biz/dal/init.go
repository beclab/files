package dal

import (
	"files/pkg/hertz/biz/dal/postgres"
	"files/pkg/hertz/biz/model/api/share"
	"k8s.io/klog/v2"
)

func migration(table interface{}, tableName string) {
	err := postgres.DB.AutoMigrate(table)
	if err != nil {
		klog.Errorf("failed to migrate database %s: %v", tableName, err)
	} else {
		klog.Infof("migrated database table %s", tableName)
	}
}

func Init() {
	postgres.Init()

	migration(&share.SharePath{}, "share_path")
	migration(&share.ShareToken{}, "share_token")
	migration(&share.ShareMember{}, "share_member")
}

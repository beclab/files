package dal

import (
	"files/pkg/hertz/biz/dal/postgres"
	"files/pkg/hertz/biz/model/api/share"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

func RebuildTable(model interface{}, tableName string) error {
	return postgres.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Migrator().DropTable(model); err != nil {
			klog.Warningf("Table %s did not exist before rebuild", tableName)
		}
		if err := tx.AutoMigrate(model); err != nil {
			klog.Errorf("Failed to create table %s: %v", tableName, err)
			return err
		}
		klog.Infof("Successfully rebuilt table %s", tableName)
		return nil
	})
}

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

	migration(&share.SharePath{}, "share_paths")
	RebuildTable(&share.ShareToken{}, "share_tokens")
	//migration(&share.ShareToken{}, "share_tokens")
	migration(&share.ShareMember{}, "share_members")
}

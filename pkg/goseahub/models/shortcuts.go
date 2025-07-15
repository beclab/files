package models

import (
	"errors"
	"gorm.io/gorm"
)

func GetFirstObjectOrNone(db *gorm.DB, out interface{}) error {
	err := db.Limit(1).First(out).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return nil
}

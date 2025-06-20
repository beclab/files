package base

import (
	"files/pkg/appdata"
	"files/pkg/constant"
	"files/pkg/drivers/base"
)

type FSStorage struct {
	Handler *base.HandlerParam
}

func (s *FSStorage) GetRoot(fsType string) string {
	switch fsType {
	case "drive", "data":
		return constant.ROOT_PREFIX
	case "cache":
		return constant.CACHE_PREFIX
	case "external":
		return constant.EXTERNAL_PREFIX
	default:
		return ""
	}
}

func (s *FSStorage) GetPvc(fsType string) (string, error) {
	switch fsType {
	case "drive", "data":
		return appdata.AppData.GetUserPVCOrCache(s.Handler.Owner)
	case "cache":
		return appdata.AppData.GetCachePVCOrCache(s.Handler.Owner)
	default:
		return "", nil
	}
}

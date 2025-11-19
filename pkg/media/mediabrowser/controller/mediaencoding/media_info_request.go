package mediaencoding

import (
	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/mediabrowser/model/dto"
)

type MediaInfoRequest struct {
	MediaSource     *dto.MediaSourceInfo
	ExtractChapters bool
	MediaType       dlna.DlnaProfileType
}

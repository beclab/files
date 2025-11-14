package playlist

type CreateMainPlaylistRequest struct {
	FilePath               string  `json:"filePath" form:"filePath" binding:"required"`
	DesiredSegmentLengthMs int     `json:"desiredSegmentLengthMs" form:"desiredSegmentLengthMs"  binding:"required"`
	TotalRuntimeTicks      int64   `json:"totalRuntimeTicks" form:"totalRuntimeTicks" binding:"required"`
	SegmentContainer       *string `json:"segmentContainer" form:"segmentContainer binding:"required"`
	EndpointPrefix         string  `json:"endpointPrefix" form:"endpointPrefix" binding:"required"`
	QueryString            string  `json:"queryString" form:"queryString" binding:"required"`
	IsRemuxingVideo        bool    `json:"isRemuxingVideo" form:"isRemuxingvideo" binding:"required"`
}

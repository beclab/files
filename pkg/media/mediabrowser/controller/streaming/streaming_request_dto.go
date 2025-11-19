package streaming

import (
	"files/pkg/media/mediabrowser/controller/mediaencoding"
)

type StreamingRequestDto struct {
	*mediaencoding.BaseEncodingJobOptions
	Params                   *string
	PlaySessionID            *string
	Tag                      *string
	SegmentContainer         *string
	SegmentLength            *int
	MinSegments              *int
	CurrentRuntimeTicks      int64
	ActualSegmentLengthTicks int64
}

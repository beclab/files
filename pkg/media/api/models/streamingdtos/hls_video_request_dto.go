package streamingdtos

import (
	"files/pkg/media/mediabrowser/controller/streaming"
)

type HlsVideoRequestDto struct {
	*streaming.VideoRequestDto
	EnableAdaptiveBitrateStreaming bool
}

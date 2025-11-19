package library

import (
	"context"
	"files/pkg/media/mediabrowser/model/dto"
	"io"
)

type ILiveStream interface {
	io.Closer
	ConsumerCount() int
	SetConsumerCount(int)
	OriginalStreamId() string
	SetOriginalStreamId(string)
	TunerHostId() string
	EnableStreamSharing() bool
	MediaSource() *dto.MediaSourceInfo
	SetMediaSource(*dto.MediaSourceInfo)
	UniqueId() string
	Open(context.Context) error
	Close() error
	GetStream() io.ReadSeeker
}

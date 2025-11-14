package library

import (
	"context"
	"files/pkg/media/mediabrowser/controller/entities"
	"files/pkg/media/mediabrowser/model/dto"
)

type IMediaSourceProvider interface {
	GetMediaSources(context.Context, entities.BaseItem) ([]dto.MediaSourceInfo, error)
	OpenMediaSource(context.Context, string, []*ILiveStream) (*ILiveStream, error)
}

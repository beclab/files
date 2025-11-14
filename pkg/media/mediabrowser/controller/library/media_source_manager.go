package library

import (
	"context"
	"github.com/google/uuid"

	en "files/pkg/media/mediabrowser/controller/entities"
	"files/pkg/media/mediabrowser/controller/persistence"
	"files/pkg/media/mediabrowser/model/dto"
	"files/pkg/media/mediabrowser/model/entities"
	"files/pkg/media/mediabrowser/model/mediainfo"
	"files/pkg/media/mediabrowser/model/mediainfo/mediaprotocol"
)

type IMediaSourceManager interface {
	AddParts(providers []IMediaSourceProvider)

	GetMediaStreams(itemId uuid.UUID) []entities.MediaStream

	GetMediaStreams2(query persistence.MediaStreamQuery) []entities.MediaStream

	GetMediaAttachments(itemId uuid.UUID) []entities.MediaAttachment

	GetMediaAttachments2(query persistence.MediaAttachmentQuery) []entities.MediaAttachment

	GetPlaybackMediaSources(item en.BaseItem /*user User,*/, allowMediaProbe, enablePathSubstitution bool, ctx context.Context) ([]dto.MediaSourceInfo, error)

	GetStaticMediaSources(item en.BaseItem, enablePathSubstitution bool /*, user *User*/) []dto.MediaSourceInfo

	GetMediaSource(item en.BaseItem, mediaSourceId, liveStreamId string, enablePathSubstitution bool, ctx context.Context) (*dto.MediaSourceInfo, error)

	OpenLiveStream(request mediainfo.LiveStreamRequest, ctx context.Context) (*mediainfo.LiveStreamResponse, error)

	OpenLiveStreamInternal(request mediainfo.LiveStreamRequest, ctx context.Context) (*mediainfo.LiveStreamResponse, IDirectStreamProvider, error)

	GetLiveStream(id string, ctx context.Context) (*dto.MediaSourceInfo, error)

	GetLiveStreamWithDirectStreamProvider(id string, ctx context.Context) (*dto.MediaSourceInfo, IDirectStreamProvider, error)

	GetLiveStreamInfo(id string) ILiveStream

	GetLiveStreamInfoByUniqueId(uniqueId string) ILiveStream

	//    GetRecordingStreamMediaSources(info ActiveRecordingInfo, ctx context.Context) ([]dto.MediaSourceInfo, error)

	CloseLiveStream(id string) error

	GetLiveStreamMediaInfo(id string, ctx context.Context) (*dto.MediaSourceInfo, error)

	SupportsDirectStream(path string, protocol mediaprotocol.MediaProtocol) bool

	GetPathProtocol(path string) mediaprotocol.MediaProtocol

	SetDefaultAudioAndSubtitleStreamIndices(item en.BaseItem, source *dto.MediaSourceInfo /*, user User*/)

	AddMediaInfoWithProbe(mediaSource *dto.MediaSourceInfo, isAudio bool, cacheKey string, addProbeDelay, isLiveStream bool, ctx context.Context) error
}

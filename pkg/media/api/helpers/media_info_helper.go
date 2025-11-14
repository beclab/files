package helpers

import (
	"context"

	"files/pkg/media/mediabrowser/controller/configuration"
	en "files/pkg/media/mediabrowser/controller/entities"
	"files/pkg/media/mediabrowser/controller/library"
	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/mediabrowser/model/dto"
	"files/pkg/media/utils"
)

import (
	"fmt"
	"time"
	// "encoding/json"
)

type IUserManager interface {
	// Add methods for IUserManager
}

type ILibraryManager interface {
	// Add methods for ILibraryManager
}

type INetworkManager interface {
	// Add methods for INetworkManager
}

type IDeviceManager interface {
	// Add methods for IDeviceManager
}

type PlaybackInfoResponse struct {
	MediaSources  []dto.MediaSourceInfo
	PlaySessionID string
	ErrorCode     *dlna.PlaybackErrorCode
}

type MediaInfoHelper struct {
	userManager                IUserManager
	libraryManager             ILibraryManager
	mediaSourceManager         library.IMediaSourceManager
	mediaEncoder               mediaencoding.IMediaEncoder
	serverConfigurationManager configuration.IServerConfigurationManager
	logger                     utils.Logger
	// networkManager             INetworkManager
	// deviceManager              IDeviceManager
}

func NewMediaInfoHelper(
	//	userManager IUserManager,
	libraryManager ILibraryManager,
	mediaSourceManager library.IMediaSourceManager,
	mediaEncoder mediaencoding.IMediaEncoder,
	serverConfigurationManager configuration.IServerConfigurationManager,
	logger utils.Logger,
	// networkManager INetworkManager,
	// deviceManager IDeviceManager,
) *MediaInfoHelper {
	return &MediaInfoHelper{
		//		userManager:                userManager,
		libraryManager:             libraryManager,
		mediaSourceManager:         mediaSourceManager,
		mediaEncoder:               mediaEncoder,
		serverConfigurationManager: serverConfigurationManager,
		logger:                     logger,
		//		networkManager:             networkManager,
		//		deviceManager:              deviceManager,
	}
}

func (m *MediaInfoHelper) GetPlaybackInfo(
	item en.BaseItem,
	//	user *User,
	mediaSourceID string,
	liveStreamID string,
) (*PlaybackInfoResponse, error) {
	result := &PlaybackInfoResponse{}

	var mediaSources []dto.MediaSourceInfo
	ctx := context.Background()
	if liveStreamID == "" {
		sources, err := m.mediaSourceManager.GetPlaybackMediaSources(item /*user,*/, true, true, ctx)
		if err != nil {
			return nil, err
		}

		if mediaSourceID == "" {
			mediaSources = sources
		} else {
			for _, source := range sources {
				if source.ID == mediaSourceID {
					mediaSources = []dto.MediaSourceInfo{source}
					break
				}
			}
		}
	} else {
		source, err := m.mediaSourceManager.GetLiveStream(liveStreamID, ctx)
		if err != nil {
			return nil, err
		}
		mediaSources = []dto.MediaSourceInfo{*source}
	}

	if len(mediaSources) == 0 {
		result.MediaSources = []dto.MediaSourceInfo{}
		result.ErrorCode = new(dlna.PlaybackErrorCode)
		*result.ErrorCode = dlna.PlaybackErrorCode(0) // No compatible stream
	} else {
		// Clone the MediaSourceInfos to avoid modifying the original ones
		clonedMediaSources := make([]dto.MediaSourceInfo, len(mediaSources))
		copy(clonedMediaSources, mediaSources)
		result.MediaSources = clonedMediaSources

		result.PlaySessionID = fmt.Sprintf("%x", time.Now().UnixNano())
	}

	return result, nil
}

type CancellationToken struct{}

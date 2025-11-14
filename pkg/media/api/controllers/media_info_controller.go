package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"

	"files/pkg/media/api/helpers"
	"files/pkg/media/api/models/mediainfodtos"
	"files/pkg/media/mediabrowser/controller/library"
	"files/pkg/media/utils"
	// "files/pkg/media/api/helpers"
)

type MediaInfoController struct {
	logger          *utils.Logger
	libraryManager  library.ILibraryManager
	mediaInfoHelper *helpers.MediaInfoHelper
}

func (m *MediaInfoController) GetPlaybackInfo(w http.ResponseWriter, r *http.Request) {
	var err error
	vars := mux.Vars(r)
	fmt.Println(vars)
	itemId, err := uuid.Parse(vars["itemId"])
	if err != nil {
		m.logger.Warnf("itemId: %v\n", err)
	} else {
		m.logger.Debugf("itemId: %v\n", itemId)
	}
	/*
	   userIDStr := r.URL.Query().Get("userId")

	   var userID *string
	   if userIDStr != "" {
	       userID = &userIDStr
	   }

	   userId := helpers.GetUserId(r, userID)
	   var user *User
	   if userId != nil {
	       user = m.userManager.GetUserById(*userId)
	   }

	   item := m.libraryManager.GetItemById(itemId, user)
	*/
	//item := m.libraryManager.GetItemById(itemId, nil)
	item := m.libraryManager.GetItemById(itemId)
	if item == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	playbackInfo, err := m.mediaInfoHelper.GetPlaybackInfo(*item /*, user*/, "", "")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(playbackInfo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (m *MediaInfoController) GetPostedPlaybackInfo(w http.ResponseWriter, r *http.Request) {
	var err error
	vars := mux.Vars(r)
	fmt.Println(vars)
	itemId, err := uuid.Parse(vars["itemId"])
	if err != nil {
		m.logger.Warnf("itemId: %v\n", err)
	} else {
		m.logger.Debugf("itemId: %v\n", itemId)
	}
	// Extract parameters from the request
	userId := r.URL.Query().Get("userId")
	maxStreamingBitrate := r.URL.Query().Get("maxStreamingBitrate")
	startTimeTicks, _ := strconv.ParseInt(r.URL.Query().Get("startTimeTicks"), 10, 64)
	audioStreamIndex := r.URL.Query().Get("audioStreamIndex")
	subtitleStreamIndex := r.URL.Query().Get("subtitleStreamIndex")
	maxAudioChannels := r.URL.Query().Get("maxAudioChannels")
	mediaSourceId := r.URL.Query().Get("mediaSourceId")
	liveStreamId := r.URL.Query().Get("liveStreamId")
	autoOpenLiveStream, _ := strconv.ParseBool(r.URL.Query().Get("autoOpenLiveStream"))
	enableDirectPlay, _ := strconv.ParseBool(r.URL.Query().Get("enableDirectPlay"))
	enableDirectStream, _ := strconv.ParseBool(r.URL.Query().Get("enableDirectStream"))
	enableTranscoding, _ := strconv.ParseBool(r.URL.Query().Get("enableTranscoding"))
	allowVideoStreamCopy, _ := strconv.ParseBool(r.URL.Query().Get("allowVideoStreamCopy"))
	allowAudioStreamCopy, _ := strconv.ParseBool(r.URL.Query().Get("allowAudioStreamCopy"))

	fmt.Println(userId, maxStreamingBitrate, startTimeTicks, audioStreamIndex, subtitleStreamIndex, maxAudioChannels, mediaSourceId, liveStreamId, autoOpenLiveStream, enableDirectPlay, enableDirectStream, enableTranscoding, allowVideoStreamCopy, allowAudioStreamCopy)

	// Decode the request body
	var playbackInfoDto mediainfodtos.PlaybackInfoDto
	err = json.NewDecoder(r.Body).Decode(&playbackInfoDto)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get the device profile
	profile := playbackInfoDto.DeviceProfile
	if profile == nil {
		/*
			caps := GetCapabilities(User.GetDeviceId())
			if caps != nil {
				profile = caps.DeviceProfile
			}
		*/
	}

	// Copy parameters from the request body
	//	userId = RequestHelpers.GetUserId(User, userId)
	maxStreamingBitrate = strconv.Itoa(*playbackInfoDto.MaxStreamingBitrate)
	startTimeTicks = *playbackInfoDto.StartTimeTicks
	audioStreamIndex = strconv.Itoa(*playbackInfoDto.AudioStreamIndex)
	subtitleStreamIndex = strconv.Itoa(*playbackInfoDto.SubtitleStreamIndex)
	maxAudioChannels = strconv.Itoa(*playbackInfoDto.MaxAudioChannels)
	mediaSourceId = *playbackInfoDto.MediaSourceId
	liveStreamId = *playbackInfoDto.LiveStreamId
	autoOpenLiveStream = *playbackInfoDto.AutoOpenLiveStream
	enableDirectPlay = *playbackInfoDto.EnableDirectPlay
	enableDirectStream = *playbackInfoDto.EnableDirectStream
	enableTranscoding = *playbackInfoDto.EnableTranscoding
	allowVideoStreamCopy = *playbackInfoDto.AllowVideoStreamCopy
	allowAudioStreamCopy = *playbackInfoDto.AllowAudioStreamCopy

	/*
		userId = helpers.GetUserId(User, userId)
		user := UserManager.GetUserById(userId)
	*/
	item := m.libraryManager.GetItemById(itemId /*, user*/)
	if item == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	info, err := m.mediaInfoHelper.GetPlaybackInfo(*item /*user,*/, mediaSourceId, liveStreamId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if info.ErrorCode != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
		return
	}

	if profile != nil {
		/*
				// Set device-specific data
				for _, mediaSource := range info.MediaSources {
					MediaInfoHelper.SetDeviceSpecificData(item, mediaSource, profile, User, maxStreamingBitrate, startTimeTicks, mediaSourceId, audioStreamIndex, subtitleStreamIndex, maxAudioChannels, info.PlaySessionId, userId, enableDirectPlay, enableDirectStream, enableTranscoding, allowVideoStreamCopy, allowAudioStreamCopy, r.RemoteAddr)
				}

				MediaInfoHelper.SortMediaSources(info, maxStreamingBitrate)
			}

			if autoOpenLiveStream {
				var mediaSource *MediaSource
				if mediaSourceId == "" {
					mediaSource = info.MediaSources[0]
				} else {
					for _, ms := range info.MediaSources {
						if ms.ID == mediaSourceId {
							mediaSource = ms
							break
						}
					}
				}

				if mediaSource != nil && mediaSource.RequiresOpening && mediaSource.LiveStreamId == "" {
					openStreamResult := MediaInfoHelper.OpenMediaSource(r.Context(), LiveStreamRequest{
						AudioStreamIndex:    audioStreamIndex,
						DeviceProfile:       playbackInfoDto.DeviceProfile,
						EnableDirectPlay:    enableDirectPlay,
						EnableDirectStream:  enableDirectStream,
						ItemId:              itemId,
						MaxAudioChannels:    maxAudioChannels,
						MaxStreamingBitrate: maxStreamingBitrate,
						PlaySessionId:       info.PlaySessionId,
						StartTimeTicks:      startTimeTicks,
						SubtitleStreamIndex: subtitleStreamIndex,
						UserId:              userId,
						OpenToken:           mediaSource.OpenToken,
					})
					info.MediaSources = []MediaSource{openStreamResult.MediaSource}
				}
		*/
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

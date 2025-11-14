package controllers

import (
	//	"context"
	"fmt"
	"log"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"

	//	"github.com/gorilla/mux"
	"net/http"
	"net/url"

	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/utils"

	"github.com/gorilla/mux"
)

type CustomPlayController struct {
	logger       *utils.Logger
	mediaEncoder mediaencoding.IMediaEncoder
}

func NewCustomPlayController(logger *utils.Logger, mediaEncoder mediaencoding.IMediaEncoder) *CustomPlayController {
	return &CustomPlayController{
		logger:       logger,
		mediaEncoder: mediaEncoder,
	}
}

func (c *CustomPlayController) Play(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	log.Printf("vars: %v", vars)
	node := vars["node"]
	log.Printf("node: %v", node)

	playPath := r.URL.Query().Get("PlayPath")
	videoBitRate := 591618 * 3
	if _, ok := r.URL.Query()["VideoBitrate"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("VideoBitrate"))
		if err == nil {
			videoBitRate = tmp
			fmt.Printf("video bitrate: %v\n", videoBitRate)
		}
	}
	audioBitRate := 128382 * 2
	if _, ok := r.URL.Query()["AudioBitrate"]; ok {
		tmp, err := strconv.Atoi(r.URL.Query().Get("AudioBitrate"))
		if err == nil {
			audioBitRate = tmp
			fmt.Printf("audio bitrate: %v\n", audioBitRate)
		}
	}
	log.Println("PlayPath:", playPath, " VideoBitrate: ", videoBitRate, " AudioBitrate: ", audioBitRate)
	playPath = filepath.Clean(playPath)
	log.Println(playPath)
	if !filepath.IsAbs(playPath) {
		http.Error(w, "invalid PlayPath", http.StatusBadRequest)
		return
	}

	/*
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		mediaInfo, err := c.mediaEncoder.GetMediaInfo(&mediaencoding.MediaInfoRequest{
			MediaSource: &dto.MediaSourceInfo{
				Protocol: mediaprotocol.File,
				Path:     playPath,
			},
			ExtractChapters: false,
			MediaType:       dlna.Video,
		}, ctx)
		if err != nil {
			c.logger.Errorf("%s\n", err)
		}
		c.logger.Infof("%+v\n", mediaInfo)
		c.logger.Infof("%d\n", *mediaInfo.RunTimeTicks)
	*/

	//itemId = uuid.Parse("83bb19e4-07a1-71d3-d254-9e997637f173")

	newURL := "/videos/" + node + "/main.m3u8?DeviceId=TW96aWxsYS81LjAgKFdpbmRvd3MgTlQgMTAuMDsgV2luNjQ7IHg2NDsgcnY6MTI2LjApIEdlY2tvLzIwMTAwMTAxIEZpcmVmb3gvMTI2LjB8MTcxNjgxMTEwMzQ4OQ11&MediaSourceId=ac78db8f57cbfe09119debf5290b9a64&VideoCodec=hevc,av1,h264,h264&AudioCodec=aac&AudioStreamIndex=1&SubtitleMethod=Encode&VideoBitrate=" + fmt.Sprint(videoBitRate) + "&AudioBitrate=" + fmt.Sprint(audioBitRate) + "&AudioSampleRate=48000&MaxFramerate=24" + "&PlaySessionId=" + uuid.New().String() + "&api_key=5e45183d4fbb4e3d98edfa478d61f104&TranscodingMaxAudioChannels=2&RequireAvc=false&Tag=ab06c8d2f35a7253bde5aef764ffafa3&SegmentContainer=mp4&MinSegments=1&BreakOnNonKeyFrames=True&h264-level=41&h264-videobitdepth=8&h264-profile=high&h264-audiochannels=2&aac-profile=lc&h264-rangetype=SDR&h264-deinterlace=true&TranscodeReasons=ContainerNotSupported" + "&PlayPath=" + url.QueryEscape(playPath)

	http.Redirect(w, r, newURL, http.StatusMovedPermanently)
}

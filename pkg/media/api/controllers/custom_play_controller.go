package controllers

import (
	//	"context"

	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/google/uuid"
	"k8s.io/klog/v2"

	//	"github.com/gorilla/mux"

	"net/http"
	"net/url"

	"files/pkg/hertz/biz/handler"
	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/utils"
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

func (c *CustomPlayController) Play(ctx context.Context, r *app.RequestContext) {
	path := string(r.Request.RequestURI())
	node := r.Param("node")

	playPath := r.Query("PlayPath")
	videoBitRate := 591618 * 3
	if _, ok := r.GetQuery("VideoBitrate"); ok {
		tmp, err := strconv.Atoi(r.Query("VideoBitrate"))
		if err == nil {
			videoBitRate = tmp
		}
	}
	audioBitRate := 128382 * 2
	if _, ok := r.GetQuery("AudioBitrate"); ok {
		tmp, err := strconv.Atoi(r.Query("AudioBitrate"))
		if err == nil {
			audioBitRate = tmp
		}
	}

	playPath = filepath.Clean(playPath)
	if !filepath.IsAbs(playPath) {
		klog.Errorf("[media] Play, invalid path: %s", playPath)
		handler.RespBadRequest(r, "invalid PlayPath")
		return
	}

	klog.Infof("[media] Play node: %s, path: %s, vBitRate: %d, aBitRate: %d, playPath: %s", node, path, videoBitRate, audioBitRate, playPath)

	newURL := "/videos/" + node + "/main.m3u8?DeviceId=TW96aWxsYS81LjAgKFdpbmRvd3MgTlQgMTAuMDsgV2luNjQ7IHg2NDsgcnY6MTI2LjApIEdlY2tvLzIwMTAwMTAxIEZpcmVmb3gvMTI2LjB8MTcxNjgxMTEwMzQ4OQ11&MediaSourceId=ac78db8f57cbfe09119debf5290b9a64&VideoCodec=hevc,av1,h264,h264&AudioCodec=aac&AudioStreamIndex=1&SubtitleMethod=Encode&VideoBitrate=" + fmt.Sprint(videoBitRate) + "&AudioBitrate=" + fmt.Sprint(audioBitRate) + "&AudioSampleRate=48000&MaxFramerate=24" + "&PlaySessionId=" + uuid.New().String() + "&api_key=5e45183d4fbb4e3d98edfa478d61f104&TranscodingMaxAudioChannels=2&RequireAvc=false&Tag=ab06c8d2f35a7253bde5aef764ffafa3&SegmentContainer=mp4&MinSegments=1&BreakOnNonKeyFrames=True&h264-level=41&h264-videobitdepth=8&h264-profile=high&h264-audiochannels=2&aac-profile=lc&h264-rangetype=SDR&h264-deinterlace=true&TranscodeReasons=ContainerNotSupported" + "&PlayPath=" + url.QueryEscape(playPath)

	r.Response.Header.Set("Location", newURL)
	r.Response.SetStatusCode(http.StatusMovedPermanently)

}

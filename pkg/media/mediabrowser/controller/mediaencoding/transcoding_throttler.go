package mediaencoding

import (
	"fmt"
	"go.uber.org/zap"
	"os"
	"time"

	cc "files/pkg/media/mediabrowser/common/configuration"
	config "files/pkg/media/mediabrowser/model/configuration"
	ioo "files/pkg/media/mediabrowser/model/io"

	"files/pkg/media/utils"
)

type TranscodingThrottler struct {
	job          *TranscodingJob
	logger       *utils.Logger
	config       cc.IConfigurationManager
	fileSystem   ioo.IFileSystem
	mediaEncoder IMediaEncoder
	ticker       *time.Ticker
	stopCh       chan struct{}
	isPaused     bool
}

func NewTranscodingThrottler(job *TranscodingJob, logger *utils.Logger, config cc.IConfigurationManager, fileSystem ioo.IFileSystem, mediaEncoder IMediaEncoder) *TranscodingThrottler {
	return &TranscodingThrottler{
		job:          job,
		logger:       logger,
		config:       config,
		fileSystem:   fileSystem,
		mediaEncoder: mediaEncoder,
	}
}

func (t *TranscodingThrottler) Start() {
	fmt.Println("TranscodingThrottler start........................")
	t.stopCh = make(chan struct{})
	t.ticker = time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-t.ticker.C:
				t.timerCallback()
			case <-t.stopCh:
				fmt.Println("Stopping ticker...")
				t.ticker.Stop()
				return
			}
		}
	}()
}

func (t *TranscodingThrottler) UnpauseTranscoding() error {
	if t.isPaused {
		t.logger.Debug("Sending resume command to ffmpeg")
		resumeKey := "u"
		if !t.mediaEncoder.IsPkeyPauseSupported() {
			resumeKey = "\n"
		}
		_, err := t.job.Stdin.Write([]byte(resumeKey))
		if err != nil {
			t.logger.Error("Error resuming transcoding", zap.Error(err))
			return err
		}
		t.isPaused = false
	}
	return nil
}

func (t *TranscodingThrottler) Stop() error {
	t.disposeTimer()
	return t.UnpauseTranscoding()
}

func (t *TranscodingThrottler) Dispose() {
	t.DisposeTimer(true)
}

func (t *TranscodingThrottler) DisposeTimer(disposing bool) {
	if disposing {
		t.disposeTimer()
	}
}

func (t *TranscodingThrottler) getOptions() *config.EncodingOptions {
	return t.config.GetEncodingOptions()
}

func (t *TranscodingThrottler) timerCallback() {
	fmt.Println("100000000000000000000000000000000000000000000000000000000000000000", *t.job.ID)
	if t.job.HasExited {
		fmt.Println("job exited...................", *t.job.PlaySessionID)
		t.disposeTimer()
		return
	}
	fmt.Println("1000000000000000000000000000000222222222222222222200000000000000000000000000000000000", *t.job.ID)

	options := t.getOptions()
	fmt.Printf("888888888888888888888888888888888888%v\n", options)
	//	options.EnableThrottling = true
	if options.EnableThrottling && t.isThrottleAllowed(t.job, max(options.ThrottleDelaySeconds, 60)) {
		err := t.pauseTranscoding()
		if err != nil {
			t.logger.Error("Error pausing transcoding", zap.Error(err))
		}
	} else {
		err := t.UnpauseTranscoding()
		if err != nil {
			t.logger.Error("Error unpausing transcoding", zap.Error(err))
		}
	}
}

func (t *TranscodingThrottler) pauseTranscoding() error {
	if !t.isPaused {
		pauseKey := "p"
		if !t.mediaEncoder.IsPkeyPauseSupported() {
			pauseKey = "c"
		}
		t.logger.Debug("Sending pause command [%s] to ffmpeg", pauseKey)
		_, err := t.job.Stdin.Write([]byte(pauseKey))
		if err != nil {
			t.logger.Error("Error pausing transcoding", zap.Error(err))
			return err
		}
		t.isPaused = true
	}
	return nil
}

func (t *TranscodingThrottler) isThrottleAllowed(job *TranscodingJob, thresholdSeconds int) bool {
	fmt.Println("qqqqqqqqqqqqqqqqqqqqqqqq")
	bytesDownloaded := job.BytesDownloaded
	transcodingPositionTicks := job.TranscodingPositionTicks
	fmt.Println(transcodingPositionTicks)
	if transcodingPositionTicks == nil {
		var ticks int64 = 0
		transcodingPositionTicks = &ticks
	}
	downloadPositionTicks := job.DownloadPositionTicks
	if downloadPositionTicks == nil {
		var ticks int64 = 0
		downloadPositionTicks = &ticks
	}

	fmt.Println("wwwwwwwwwwwwwwwwwwwww")
	path := job.Path
	if path == nil || *path == "" {
		return false
	}

	fmt.Println("eeeeeeeeeeeeeeeeeeeeeeee")
	gapLengthInTicks := (time.Duration(thresholdSeconds) * time.Second).Nanoseconds() / 100
	fmt.Println(gapLengthInTicks)
	fmt.Println(*downloadPositionTicks)
	fmt.Println(*transcodingPositionTicks)

	if *downloadPositionTicks > 0 && *transcodingPositionTicks > 0 {
		fmt.Println("rrrrrrrrrrrrrrrrrrrrrrrrrrrrrrr")
		// HLS - time-based consideration
		targetGap := gapLengthInTicks
		gap := *transcodingPositionTicks - *downloadPositionTicks
		if gap < targetGap {
			t.logger.Debugf("Not throttling transcoder gap %d target gap %d", gap, targetGap)
			return false
		}
		t.logger.Debugf("Throttling transcoder gap %d target gap %d", gap, targetGap)
		return true
	}

	if bytesDownloaded > 0 && *transcodingPositionTicks > 0 {
		// Progressive Streaming - byte-based consideration
		var bytesTranscoded int64
		fileInfo, err := os.Stat(*path)
		if err != nil {
			bytesTranscoded = *job.BytesTranscoded
			if bytesTranscoded == 0 {
				bytesTranscoded = fileInfo.Size()
			}
		}

		// Estimate the bytes the transcoder should be ahead
		gapFactor := float64(gapLengthInTicks) / float64(*transcodingPositionTicks)
		targetGap := int64(float64(bytesTranscoded) * gapFactor)

		gap := bytesTranscoded - bytesDownloaded
		if gap < targetGap {
			t.logger.Debugf("Not throttling transcoder gap %d target gap %d bytes downloaded %d", gap, targetGap, bytesDownloaded)
			return false
		}

		t.logger.Debugf("Throttling transcoder gap %d target gap %d bytes downloaded %d", gap, targetGap, bytesDownloaded)
		return true
	}

	t.logger.Debugf("No throttle data for %s\n", *path)
	return false
}

func (t *TranscodingThrottler) disposeTimer() {
	if t.stopCh != nil {
		fmt.Println("ticker stop...................", *t.job.PlaySessionID)
		close(t.stopCh)
		t.stopCh = nil
		fmt.Println("Disposed timer")
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

package mediaencoding

import (
	//	"context"
	"fmt"
	"math"
	"strings"
	//	"io/ioutil"
	//	"os"
	"path/filepath"
	"strconv"
	"time"

	cc "files/pkg/media/mediabrowser/common/configuration"
	"files/pkg/media/mediabrowser/model/configuration"
	ioo "files/pkg/media/mediabrowser/model/io"
	"files/pkg/media/utils"

	"k8s.io/klog/v2"
)

/*
type ILogger interface {
	LogDebug(format string, v ...interface{})
	LogError(format string, v ...interface{})
}
*/

type TranscodingSegmentCleaner struct {
	job           *TranscodingJob
	logger        *utils.Logger
	config        cc.IConfigurationManager
	fileSystem    ioo.IFileSystem
	mediaEncoder  IMediaEncoder
	segmentLength int
	ticker        *time.Ticker
	stopCh        chan struct{}
}

func NewTranscodingSegmentCleaner(job *TranscodingJob, logger *utils.Logger, config cc.IConfigurationManager, fileSystem ioo.IFileSystem, mediaEncoder IMediaEncoder, segmentLength int) *TranscodingSegmentCleaner {
	return &TranscodingSegmentCleaner{
		job:           job,
		logger:        logger,
		config:        config,
		fileSystem:    fileSystem,
		mediaEncoder:  mediaEncoder,
		segmentLength: segmentLength,
	}
}

func (t *TranscodingSegmentCleaner) Start() {
	klog.Infoln("Starting TranscodingSegmentCleaner")
	t.stopCh = make(chan struct{})
	t.ticker = time.NewTicker(20 * time.Second)

	go func() {
		for {
			select {
			case <-t.ticker.C:
				t.timerCallback(nil)
			case <-t.stopCh:
				klog.Infoln("Stopping ticker...")
				t.ticker.Stop()
				return
			}
		}
	}()
}

func (t *TranscodingSegmentCleaner) Stop() {
	t.DisposeTimer()
}

func (t *TranscodingSegmentCleaner) Dispose() {
	t.Dispose2(true)
}

func (t *TranscodingSegmentCleaner) Dispose2(disposing bool) {
	if disposing {
		t.DisposeTimer()
	}
}

func (t *TranscodingSegmentCleaner) timerCallback(state interface{}) {
	klog.Infoln("timerCallback-1 ", *t.job.ID)
	if t.job.HasExited {
		t.DisposeTimer()
		return
	}
	klog.Infoln("timerCallback-2 ", *t.job.ID)

	options := t.GetOptions()
	//	options.EnableSegmentDeletion = true
	enableSegmentDeletion := options.EnableSegmentDeletion
	segmentKeepSeconds := int64(math.Max(float64(options.SegmentKeepSeconds), 20))

	if enableSegmentDeletion {
		downloadPositionTicks := t.job.DownloadPositionTicks
		if downloadPositionTicks != nil {
			downloadPositionSeconds := int64(time.Duration(*downloadPositionTicks * 100).Seconds())

			if downloadPositionSeconds > 0 && segmentKeepSeconds > 0 && downloadPositionSeconds > segmentKeepSeconds {
				idxMaxToDelete := (downloadPositionSeconds - segmentKeepSeconds) / int64(t.segmentLength)

				if idxMaxToDelete > 0 {
					t.DeleteSegmentFiles(t.job, 0, idxMaxToDelete, 1500)
				}
			}
		}
	}
}

func (t *TranscodingSegmentCleaner) GetOptions() *configuration.EncodingOptions {
	return t.config.GetEncodingOptions()
}

func (t *TranscodingSegmentCleaner) DeleteSegmentFiles(job *TranscodingJob, idxMin, idxMax int64, delayMs int) error {
	path := job.Path
	if path == nil {
		return fmt.Errorf("path can't be null")
	}

	//t.logger.LogDebug("Deleting segment file(s) index %d to %d from %s", idxMin, idxMax, *path)
	klog.Infof("Deleting segment file(s) index %d to %d from %s\n", idxMin, idxMax, *path)

	time.Sleep(time.Duration(delayMs) * time.Millisecond)

	switch job.Type {
	case Hls:
		if err := t.DeleteHlsSegmentFiles(*path, idxMin, idxMax); err != nil {
			//t.logger.LogDebug("error deleting segment file(s) %s: %v", *path, err)
			klog.Infof("error deleting segment file(s) %s: %v\n", *path, err)
			return err
		}
	default:
		// Handle other types of transcoding jobs
	}

	return nil
}

func (t *TranscodingSegmentCleaner) DeleteHlsSegmentFiles(outputFilePath string, idxMin, idxMax int64) error {
	directory := filepath.Dir(outputFilePath)
	if directory == "." {
		return fmt.Errorf("path can't be a root directory: %s", outputFilePath)
	}

	name := filepath.Base(outputFilePath)
	name = name[:len(name)-len(filepath.Ext(name))]

	var errs []error
	files, err := t.fileSystem.GetFilePaths(directory, false)
	if err != nil {
		klog.Infoln(err)
		return err
	}

	for _, file := range files {
		fileName := filepath.Base(file)
		if !strings.HasPrefix(fileName, name) {
			continue
		}
		fileName = fileName[:len(fileName)-len(filepath.Ext(fileName))]
		idx, err := strconv.ParseInt(fileName[len(name):], 10, 64)
		if err != nil {
			klog.Infoln("transcoding_segment_cleaner", err, fileName, fileName[len(name):], "<-")
		}

		if err == nil && idx >= idxMin && idx <= idxMax {
			//c.logger.LogDebug("Deleting HLS segment file %s", file)
			klog.Infof("Deleting HLS segment file %s\n", file)
			if err := t.fileSystem.DeleteFile(file); err != nil {
				errs = append(errs, err)
				//t.logger.LogDebug("Error deleting HLS segment file %s: %v", file, err)
				klog.Infof("Error deleting HLS segment file %s: %v\n", file, err)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("error deleting HLS segment files: %v", errs)
	}
	return nil
}

func (t *TranscodingSegmentCleaner) DisposeTimer() {
	if t.stopCh != nil {
		close(t.stopCh)
		t.stopCh = nil
		klog.Infoln("Disposed timer")
	}
}

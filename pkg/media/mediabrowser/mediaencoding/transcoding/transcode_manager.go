package transcoding

import (
	"context"
	"fmt"

	//	"os/exec"
	"os"
	//	"crypto/md5"
	"encoding/json"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"files/pkg/media/mediabrowser/controller/configuration"
	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/mediabrowser/controller/mediaencoding/joblogger"
	//	"files/pkg/media/mediabrowser/controller/mediaencoding/transcodemanager"
	cs "files/pkg/media/mediabrowser/controller/session"
	"files/pkg/media/mediabrowser/controller/streaming"
	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/mediabrowser/model/entities"
	ioo "files/pkg/media/mediabrowser/model/io"
	"files/pkg/media/mediabrowser/model/mediainfo/mediaprotocol"
	"files/pkg/media/mediabrowser/model/session"
	"files/pkg/media/utils"

	"k8s.io/klog/v2"
)

// IDisposable represents a disposable resource.
type IDisposable interface {
	Dispose()
}

type TranscodeManager struct {
	jobs                       map[string]*mediaencoding.TranscodingJob
	logger                     *utils.Logger
	activeTranscodingJobs      []*mediaencoding.TranscodingJob
	activeTranscodingJobsLock  sync.RWMutex
	transcodingLocks           *utils.AsyncKeyedLocker
	mediaEncoder               mediaencoding.IMediaEncoder
	fileSystem                 ioo.IFileSystem
	serverConfigurationManager configuration.IServerConfigurationManager
	sessionManager             cs.ISessionManager
}

func NewTranscodeManager(mediaEncoder mediaencoding.IMediaEncoder, fileSystem ioo.IFileSystem, serverConfigurationManager configuration.IServerConfigurationManager, sessionManager cs.ISessionManager, logger *utils.Logger) *TranscodeManager {
	t := &TranscodeManager{
		jobs:   make(map[string]*mediaencoding.TranscodingJob),
		logger: logger,
		transcodingLocks: utils.NewAsyncKeyedLocker(utils.AsyncKeyedLockerConfig{
			PoolSize:        20,
			PoolInitialFill: 1,
		}),
		mediaEncoder:               mediaEncoder,
		fileSystem:                 fileSystem,
		serverConfigurationManager: serverConfigurationManager,
		sessionManager:             sessionManager,
	}
	t.DeleteEncodedMediaCache()

	return t
}

func (m *TranscodeManager) GetTranscodingJob(playSessionID string) *mediaencoding.TranscodingJob {
	m.activeTranscodingJobsLock.RLock()
	defer m.activeTranscodingJobsLock.RUnlock()
	for _, job := range m.activeTranscodingJobs {
		if strings.EqualFold(*job.PlaySessionID, playSessionID) {
			return job
		}
	}
	return nil
}

func (m *TranscodeManager) GetTranscodingJob2(path string, jobType mediaencoding.TranscodingJobType) *mediaencoding.TranscodingJob {
	m.activeTranscodingJobsLock.RLock()
	defer m.activeTranscodingJobsLock.RUnlock()
	for _, job := range m.activeTranscodingJobs {
		klog.Infoln(path, *job.Path, job.Type, jobType)
		if *job.Path == path && job.Type == jobType {
			return job
		}
	}
	return nil
}

func (m *TranscodeManager) PingTranscodingJob(playSessionID string, isUserPaused *bool) {
	if playSessionID == "" {
		panic("playSessionID cannot be empty")
	}

	m.logger.Debugf("PingTranscodingJob PlaySessionID=%s isUserPaused: %v", playSessionID, isUserPaused)

	m.activeTranscodingJobsLock.RLock()
	defer m.activeTranscodingJobsLock.RUnlock()

	// This is really only needed for HLS.
	// Progressive streams can stop on their own reliably.
	var jobs []*mediaencoding.TranscodingJob
	for _, job := range m.activeTranscodingJobs {
		if strings.EqualFold(playSessionID, *job.PlaySessionID) {
			jobs = append(jobs, job)
		}
	}

	for _, job := range jobs {
		if isUserPaused != nil {
			m.logger.Debugf("Setting job.IsUserPaused to %t. jobID: %s", *isUserPaused, job.ID)
			job.IsUserPaused = *isUserPaused
		}
		m.pingTimer(job, true)
	}
}

func (m *TranscodeManager) pingTimer(job *mediaencoding.TranscodingJob, isProgressCheckIn bool) {
	if job.HasExited {
		job.StopKillTimer()
		return
	}

	timerDuration := 10 * time.Second
	if job.Type != mediaencoding.Progressive {
		timerDuration = 60 * time.Second * 5
	}

	job.PingTimeout = timerDuration
	job.LastPingDate = time.Now().UTC()

	// Don't start the timer for playback checkins with progressive streaming
	if job.Type != mediaencoding.Progressive || !isProgressCheckIn {
		job.StartKillTimer(m.onTranscodeKillTimerStopped)
	} else {
		job.ChangeKillTimerIfStarted()
	}
}

func (m *TranscodeManager) onTranscodeKillTimerStopped(state interface{}) {
	job, ok := state.(*mediaencoding.TranscodingJob)
	if !ok {
		m.logger.Errorf("state is not of type *TranscodingJob")
		return
	}

	if !job.HasExited && job.Type != mediaencoding.Progressive {
		timeSinceLastPing := time.Since(job.LastPingDate).Milliseconds()
		if timeSinceLastPing < job.PingTimeout.Milliseconds() {
			job.StartKillTimerWithTimeout(m.onTranscodeKillTimerStopped, job.PingTimeout)
			return
		}
	}

	m.logger.Infof("Transcoding kill timer stopped for JobId %d PlaySessionId %d. Killing transcoding", job.ID, job.PlaySessionID)
	m.killTranscodingJob(job, true, func(path string) bool { return true })
}

func (m *TranscodeManager) KillTranscodingJobs(deviceID, playSessionID string, deleteFiles func(string) bool) error {
	klog.Infoln("kill..................................")
	var jobs []*mediaencoding.TranscodingJob
	{
		m.activeTranscodingJobsLock.RLock()
		defer m.activeTranscodingJobsLock.RUnlock()

		// This is really only needed for HLS.
		// Progressive streams can stop on their own reliably.
		for _, job := range m.activeTranscodingJobs {
			if playSessionID == "" {
				if strings.EqualFold(deviceID, *job.DeviceID) {
					klog.Infoln("............DeviceID................")
					jobs = append(jobs, job)
				}
			} else if strings.EqualFold(playSessionID, *job.PlaySessionID) {
				klog.Infoln("............PlaySessionID................")
				jobs = append(jobs, job)
			}
		}
	}

	klog.Infoln("kill..................................job len ", len(jobs))
	var wg sync.WaitGroup
	for _, job := range jobs {
		wg.Add(1)
		go func(j *mediaencoding.TranscodingJob) {
			defer wg.Done()
			klog.Infoln("kill..............job....................")
			klog.Infof("%+v\n", j)
			err := m.killTranscodingJob(j, false, deleteFiles)
			if err != nil {
				m.logger.Errorf("Error killing transcoding job: %v", err)
			}
		}(job)
	}

	wg.Wait()
	return nil
}

func (t *TranscodeManager) StartFfMpeg(state streaming.StreamState, outputPath, commandLineArguments string, userID string,
	transcodingJobType mediaencoding.TranscodingJobType, cancellationTokenSource context.Context, workingDir string) (*mediaencoding.TranscodingJob, error) {
	klog.Infoln("StartFfMpeg !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!")
	directory := filepath.Dir(outputPath)
	if directory == "" {
		return nil, fmt.Errorf("provided path (%s) is not valid", outputPath)
	}

	err := os.MkdirAll(directory, 0755)
	if err != nil {
		return &mediaencoding.TranscodingJob{}, err
	}

	err = t.AcquireResources(&state, cancellationTokenSource)
	if err != nil {
		return &mediaencoding.TranscodingJob{}, err
	}

	/*
		if state.VideoRequest != nil && !mediaencoding.IsCopyCodec(state.OutputVideoCodec) {
			user, err := _userManager.GetUserByID(userID)
			if err != nil || (user != nil && !user.HasPermission(PermissionKind.EnableVideoPlaybackTranscoding)) {
				t.OnTranscodeFailedToStart(outputPath, transcodingJobType, &state)
				return &mediaencoding.TranscodingJob{}, fmt.Errorf("user does not have access to video transcoding")
			}
		}
	*/

	if t.mediaEncoder.EncoderPath() == "" {
		return &mediaencoding.TranscodingJob{}, fmt.Errorf("media encoder path is not set")
	}

	if state.SubtitleStream != nil && state.SubtitleDeliveryMethod == dlna.Encode {
		// klog.Infoln("comment.........")
		/*
		   attachmentPath := filepath.Join(_appPaths.CachePath, "attachments", state.MediaSource.ID)
		   if *state.MediaSource.VideoType == entities.Dvd || *state.MediaSource.VideoType == entities.BluRay {
		       concatPath := filepath.Join(_serverConfigurationManager.GetTranscodePath(), state.MediaSource.ID+".concat")
		       err = _attachmentExtractor.ExtractAllAttachments(concatPath, state.MediaSource, attachmentPath, cancellationTokenSource)
		       if err != nil {
		           return &mediaencoding.TranscodingJob{}, err
		       }
		   } else {
		       err = _attachmentExtractor.ExtractAllAttachments(state.MediaPath, state.MediaSource, attachmentPath, cancellationTokenSource)
		       if err != nil {
		           return &mediaencoding.TranscodingJob{}, err
		       }
		   }

		   if state.SubtitleStream.IsExternal && strings.EqualFold(filepath.Ext(state.SubtitleStream.Path), ".mks") {
		       subtitlePath := state.SubtitleStream.Path
		       subtitlePathArgument := fmt.Sprintf("file:\"%s\"", strings.ReplaceAll(subtitlePath, "\"", "\\\""))
		       subtitleID := fmt.Sprintf("%x", md5.Sum([]byte(subtitlePath)))
		       err = _attachmentExtractor.ExtractAllAttachmentsExternal(subtitlePathArgument, subtitleID, attachmentPath, cancellationTokenSource)
		       if err != nil {
		           return &mediaencoding.TranscodingJob{}, err
		       }
		   }
		*/
	}

	/*
		    process := exec.Command(_mediaEncoder.EncoderPath, strings.Split(commandLineArguments, " ")...)
		process.SysProcAttr = &syscall.SysProcAttr{
		    CreationFlags: syscall.CREATE_NO_WINDOW,
		}
		process.Stdout = os.Stdout
		process.Stderr = os.Stderr
		process.Stdin = os.Stdin
		process.Dir = workingDirectory
	*/
	//	var process = utils.NewProcess(t.mediaEncoder.EncoderPath(), strings.Split(commandLineArguments, " ")...)

	var process = utils.NewProcess("sh", "-c", t.mediaEncoder.EncoderPath()+" "+commandLineArguments)

	transcodingJob := t.OnTranscodeBeginning(
		outputPath,
		state.Request.PlaySessionID,
		&state.MediaSource.LiveStreamID,
		uuid.NewString(),
		transcodingJobType,
		process,
		&state.Request.DeviceID,
		&state,
		cancellationTokenSource,
	)

	t.logger.Infof("> %s %s", process.ProcessName(), commandLineArguments)

	logFilePrefix := "FFmpeg.Transcode-"
	if state.VideoRequest != nil && mediaencoding.IsCopyCodec(state.OutputVideoCodec) {
		if mediaencoding.IsCopyCodec(state.OutputAudioCodec) {
			logFilePrefix = "FFmpeg.Remux-"
		} else {
			logFilePrefix = "FFmpeg.DirectStream-"
		}
	}

	logFilePath := filepath.Join(
		//    _serverConfigurationManager.ApplicationPaths.LogDirectoryPath,
		"/tmp/log",
		fmt.Sprintf("%s%s_%s_%s.log", logFilePrefix, time.Now().Format("2006-01-02_15-04-05"), state.Request.MediaSourceID, uuid.NewString()[:8]),
	)

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.logger.Errorf("Error opening log file: %v", err)
		return &mediaencoding.TranscodingJob{}, err
	}
	//	defer logFile.Close()

	enc := json.NewEncoder(logFile)
	err = enc.Encode(state.MediaSource)
	if err != nil {
		t.logger.Errorf("Error writing media source to log file: %v", err)
		return &mediaencoding.TranscodingJob{}, err
	}

	commandLineLogMessage := []byte(fmt.Sprintf("\n\n%s %s\n\n", process.ProcessName(), commandLineArguments))
	_, err = logFile.Write(commandLineLogMessage)
	if err != nil {
		t.logger.Errorf("Error writing command line to log file: %v", err)
		return &mediaencoding.TranscodingJob{}, err
	}

	stderr, err := process.StderrPipe()
	if err != nil {
		return nil, err
	}

	err = process.Start()
	if err != nil {
		t.logger.Errorf("Error starting FFmpeg: %v", err)
		t.OnTranscodeFailedToStart(outputPath, transcodingJobType, &state)
		return nil, err
	}

	t.logger.Debug("Launched FFmpeg process")

	go func() {
		err := process.Wait()
		if err != nil {
			t.logger.Errorf("Error Wait FFmpeg: %v", err)
		}
		t.OnFfMpegProcessExited(process, transcodingJob, &state)
		/*
			if err != nil {
				if exitError, ok := err.(*exec.ExitError); ok {
					t.OnFfMpegProcessExited(process, transcodingJob, state, exitError.ExitCode())
				} else {
					t.OnFfMpegProcessExited(process, transcodingJob, state, -1)
				}
			} else {
				t.OnFfMpegProcessExited(process, transcodingJob, state, 0)
			}
		*/
	}()

	state.TranscodingJob = transcodingJob

	// Important - don't await the log task or we won't be able to kill FFmpeg when the user stops playback
	go joblogger.NewJobLogger().StartStreamingLog(state, stderr, logFile)

	// Wait for the file to exist before proceeding
	ffmpegTargetFile := state.WaitForPath
	if ffmpegTargetFile == "" {
		ffmpegTargetFile = outputPath
	}
	t.logger.Debugf("Waiting for the creation of %s", ffmpegTargetFile)
	for !fileExists(ffmpegTargetFile) && !transcodingJob.HasExited {
		time.Sleep(100 * time.Millisecond)
	}

	t.logger.Debugf("File %s created or transcoding has finished", ffmpegTargetFile)

	if state.IsInputVideo && transcodingJob.Type == mediaencoding.Progressive && !transcodingJob.HasExited {
		time.Sleep(1 * time.Second)

		if state.ReadInputAtNativeFramerate && !transcodingJob.HasExited {
			time.Sleep(1500 * time.Millisecond)
		}
	}

	if !transcodingJob.HasExited {
		t.StartThrottler(state, transcodingJob)
		t.StartSegmentCleaner(state, transcodingJob)
	} else if transcodingJob.ExitCode != 0 {
		return nil, fmt.Errorf("FFmpeg exited with code %d", transcodingJob.ExitCode)
	}

	t.logger.Debug("StartFfMpeg() finished successfully")

	return transcodingJob, nil
}

func (m *TranscodeManager) killTranscodingJob(job *mediaencoding.TranscodingJob, closeLiveStream bool, deleteFiles func(string) bool) error {
	job.DisposeKillTimer()

	m.logger.Debugf("KillTranscodingJob - JobID %s PlaySessionID %s. Killing transcoding", *job.ID, *job.PlaySessionID)

	{
		m.activeTranscodingJobsLock.RLock()
		defer m.activeTranscodingJobsLock.RUnlock()

		for i, activeJob := range m.activeTranscodingJobs {
			if activeJob == job {
				m.activeTranscodingJobs = append(m.activeTranscodingJobs[:i], m.activeTranscodingJobs[i+1:]...)
				break
			}
		}
		/* to do
		if job.CancellationTokenSource != nil && !job.CancellationTokenSource.IsCancellationRequested() {
			job.CancellationTokenSource.Cancel()
		}
		*/
	}
	job.Stop()

	if deleteFiles(*job.Path) {
		err := m.DeletePartialStreamFiles(*job.Path, job.Type, 0, 1500)
		if err != nil {
			m.logger.Errorf("Error deleting partial stream files: %v", err)
		}
		if job.MediaSource != nil && (*job.MediaSource.VideoType == entities.Dvd || *job.MediaSource.VideoType == entities.BluRay) {
			/*
				concatFilePath := filepath.Join(m.serverConfigManager.GetTranscodePath(), job.MediaSource.ID+".concat")
				if _, err := os.Stat(concatFilePath); err == nil {
					m.logger.Infof("Deleting ffmpeg concat configuration at %s", concatFilePath)
					err = os.Remove(concatFilePath)
					if err != nil {
						m.logger.Errorf("Error deleting ffmpeg concat configuration: %v", err)
					}
				}
			*/
		}
	}

	if closeLiveStream && *job.LiveStreamID != "" {
		/*
			err := m.mediaSourceManager.closeLiveStream(job.LiveStreamID)
			if err != nil {
				m.logger.Errorf("Error closing live stream for %s: %v", job.Path, err)
			}
		*/
	}

	return nil
}

func (m *TranscodeManager) ReportTranscodingProgress(job *mediaencoding.TranscodingJob, state *streaming.StreamState, transcodingPosition *time.Duration, framerate *float32, percentComplete *float64, bytesTranscoded *int64, bitRate *int) {

	var ticks int64
	if transcodingPosition != nil {
		ticks = transcodingPosition.Nanoseconds() / 100
		klog.Infoln(ticks)
	}

	if job != nil {
		job.Framerate = framerate
		job.CompletionPercentage = percentComplete
		klog.Infoln(ticks)
		job.TranscodingPositionTicks = &ticks
		job.BytesTranscoded = bytesTranscoded
		job.BitRate = bitRate
	}

	deviceID := state.GetRequest().DeviceID
	if deviceID != "" {
		audioCodec := state.ActualOutputAudioCodec()
		videoCodec := state.ActualOutputVideoCodec()
		hardwareAccelerationType := m.serverConfigurationManager.GetEncodingOptions().HardwareAccelerationType

		/*
			var transcodeReasons string
			if state.TranscodeReasons != nil {
				transcodeReasons = *state.TranscodeReasons
			}
		*/

		m.sessionManager.ReportTranscodingInfo(deviceID, &session.TranscodingInfo{
			Bitrate:                  bitRate,
			AudioCodec:               audioCodec,
			VideoCodec:               videoCodec,
			Container:                state.OutputContainer,
			Framerate:                framerate,
			CompletionPercentage:     percentComplete,
			Width:                    state.OutputWidth,
			Height:                   state.OutputHeight(),
			AudioChannels:            state.OutputAudioChannels,
			IsAudioDirect:            mediaencoding.IsCopyCodec(state.OutputAudioCodec),
			IsVideoDirect:            mediaencoding.IsCopyCodec(state.OutputVideoCodec),
			HardwareAccelerationType: &hardwareAccelerationType,
			TranscodeReasons:         0,
		})
	}
}

func (m *TranscodeManager) OnTranscodeEndRequest(job *mediaencoding.TranscodingJob) {
	job.ActiveRequestCount--
	m.logger.Debug("OnTranscodeEndRequest job.ActiveRequestCount=%d", job.ActiveRequestCount)
	if job.ActiveRequestCount <= 0 {
		m.pingTimer(job, false)
	}
}

func (m *TranscodeManager) OnTranscodeBeginRequest(path string, jobType mediaencoding.TranscodingJobType) *mediaencoding.TranscodingJob {
	m.activeTranscodingJobsLock.RLock()
	defer m.activeTranscodingJobsLock.RUnlock()

	for _, job := range m.activeTranscodingJobs {
		if job.Type == jobType && strings.EqualFold(*job.Path, path) {
			job.ActiveRequestCount++
			if *job.PlaySessionID == "" || job.Type == mediaencoding.Progressive {
				job.StopKillTimer()
			}
			return job
		}
	}

	return nil
}

//
//func (m *TranscodeManager) onPlaybackProgress(args *PlaybackProgressEventArgs) {
//	if args.PlaySessionID != "" {
//		m.pingTranscodingJob(args.PlaySessionID, args.IsPaused)
//	}
//}
//
//func (m *TranscodeManager) deleteEncodedMediaCache() {
//	path := m.serverConfigurationManager.getTranscodePath()
//	if !s.fileSystem.dirExists(path) {
//		return
//	}
//
//	files, err := m.fileSystem.getFilePaths(path, true)
//	if err != nil {
//		s.logger.logError(err, "Error getting file paths for encoded media cache directory: %s", path)
//		return
//	}
//
//	for _, file := range files {
//		err := m.fileSystem.deleteFile(file)
//		if err != nil {
//			m.logger.logError(err, "Error deleting encoded media cache file: %s", file)
//		}
//	}
//}
//

// func (m *TranscodeManager) LockAsync(outputPath string, cancellationToken context.Context) (IDisposable, error) {
func (m *TranscodeManager) LockAsync(outputPath string, cancellationToken context.Context) (func(), error) {
	return m.transcodingLocks.LockAsync(cancellationToken, outputPath)
}

//func (m *TranscodeManager) LockAsync(outputPath string, cancelChan <-chan struct{}) (func(), error) {
//return nil, nil
//lockCtx, cancelLock := context.WithCancel(context.Background())
//defer cancelLock()

//lock, err := m.transcodingLocks.lock(outputPath, lockCtx)
//if err != nil {
//	return nil, err
//}
//
//select {
//case <-cancelChan:
//	lock.Unlock()
//	return nil, context.Canceled
//default:
//	return func() {
//		lock.Unlock()
//	}, nil
//}

//
//func (m *TranscodeManager) Dispose() {
//	m.sessionManager.unsubscribePlaybackProgress(m.onPlaybackProgress)
//	m.sessionManager.unsubscribePlaybackStart(m.onPlaybackProgress)
//	m.transcodingLocks.dispose()
//}
//

func (m *TranscodeManager) DeletePartialStreamFiles(path string, jobType mediaencoding.TranscodingJobType, retryCount int, delayMs int) error {
	if retryCount >= 10 {
		return nil
	}
	/*
	   select {
	   case <-ctx.Done():
	       return ctx.Err()
	   case <-time.After(time.Duration(delayMs) * time.Millisecond):
	   }
	*/

	m.logger.Infof("Deleting partial stream file(s) %s", path)

	time.Sleep(time.Duration(delayMs) * time.Millisecond)

	err := func() error {
		if jobType == mediaencoding.Progressive {
			return m.DeleteProgressivePartialStreamFiles(path)
		} else {
			return m.DeleteHlsPartialStreamFiles(path)
		}
	}()

	if err != nil {
		m.logger.Errorf("Error deleting partial stream file(s) %s: %v", path, err)
		m.DeletePartialStreamFiles(path, jobType, retryCount+1, 500)
	}

	return nil
}

func (t *TranscodeManager) AcquireResources(state *streaming.StreamState, cancellationTokenSource context.Context) error {
	/*
	   if state.MediaSource.RequiresOpening && state.Request.LiveStreamId == "" {
	       liveStreamResponse, err := t.mediaSourceManager.OpenLiveStream(
	           &mediainfo.LiveStreamRequest{OpenToken: state.MediaSource.OpenToken},
	           cancellationTokenSource.Token,
	       )
	       if err != nil {
	           return err
	       }

	       encodingOptions := t.serverConfigurationManager.GetEncodingOptions()
	       t.encodingHelper.AttachMediaSourceInfo(state, encodingOptions, liveStreamResponse.MediaSource, state.RequestedUrl)

	       if state.VideoRequest != nil {
	           t.encodingHelper.TryStreamCopy(state)
	       }
	   }

	   if state.MediaSource.BufferMs != nil {
	       select {
	       case <-time.After(time.Duration(*state.MediaSource.BufferMs) * time.Millisecond):
	       case <-cancellationTokenSource.Done():
	           return cancellationTokenSource.Err()
	       }
	   }
	*/

	return nil
}

func (t *TranscodeManager) OnTranscodeFailedToStart(path string, jobType mediaencoding.TranscodingJobType, state *streaming.StreamState) {
	t.activeTranscodingJobsLock.RLock()
	defer t.activeTranscodingJobsLock.RUnlock()

	for i, job := range t.activeTranscodingJobs {
		if job.Type == jobType && strings.EqualFold(*job.Path, path) {
			t.activeTranscodingJobs = append(t.activeTranscodingJobs[:i], t.activeTranscodingJobs[i+1:]...)
			break
		}
	}

	if state.Request.DeviceID != "" {
		t.sessionManager.ClearTranscodingInfo(state.Request.DeviceID)
	}
}

func (t *TranscodeManager) OnTranscodeBeginning(
	path string,
	playSessionID *string,
	liveStreamID *string,
	transcodingJobID string,
	jobType mediaencoding.TranscodingJobType,
	process *utils.Process,
	deviceID *string,
	state *streaming.StreamState,
	ctx context.Context,
) *mediaencoding.TranscodingJob {
	t.activeTranscodingJobsLock.RLock()
	defer t.activeTranscodingJobsLock.RUnlock()

	stdin, err := process.StdinPipe()
	if err != nil {
		return nil
	}

	job := &mediaencoding.TranscodingJob{
		Logger:  t.logger,
		Type:    jobType,
		Path:    &path,
		Stdin:   stdin,
		Process: process,
		//ActiveRequestCount:      1,
		ActiveRequestCount:      0,
		DeviceID:                deviceID,
		CancellationTokenSource: &ctx,
		ID:                      &transcodingJobID,
		PlaySessionID:           playSessionID,
		LiveStreamID:            liveStreamID,
		MediaSource:             state.MediaSource,
	}

	t.activeTranscodingJobs = append(t.activeTranscodingJobs, job)

	t.ReportTranscodingProgress(job, state, nil, nil, nil, nil, nil)

	return job
}

func (t *TranscodeManager) OnFfMpegProcessExited(process *utils.Process, job *mediaencoding.TranscodingJob, state *streaming.StreamState) {
	job.HasExited = true
	job.ExitCode = process.ExitCode()

	t.ReportTranscodingProgress(job, state, nil, nil, nil, nil, nil)

	t.logger.Debugf("Disposing stream resources")
	state.Dispose()

	if process.ExitCode() == 0 {
		t.logger.Infof("FFmpeg exited with code 0")
	} else {
		t.logger.Errorf("FFmpeg exited with code %d", process.ExitCode())
	}

	job.Dispose()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (t *TranscodeManager) StartThrottler(state streaming.StreamState, transcodingJob *mediaencoding.TranscodingJob) {
	if t.EnableThrottling(state) {
		transcodingJob.TranscodingThrottler = mediaencoding.NewTranscodingThrottler(
			transcodingJob,
			t.logger,

			//            t.loggerFactory.CreateLogger("TranscodingThrottler"),
			t.serverConfigurationManager,
			t.fileSystem,
			t.mediaEncoder,
		)
		transcodingJob.TranscodingThrottler.Start()
	}
}

func (t *TranscodeManager) EnableThrottling(state streaming.StreamState) bool {
	return state.InputProtocol == mediaprotocol.File &&
		state.RunTimeTicks != nil &&
		*state.RunTimeTicks >= time.Minute.Nanoseconds()*5/100 &&
		state.IsInputVideo &&
		state.VideoType == entities.VideoFile
}

func (t *TranscodeManager) StartSegmentCleaner(state streaming.StreamState, transcodingJob *mediaencoding.TranscodingJob) {
	if t.EnableSegmentCleaning(state) {
		transcodingJob.TranscodingSegmentCleaner = mediaencoding.NewTranscodingSegmentCleaner(
			transcodingJob,
			t.logger,
			//            t.loggerFactory.CreateLogger("TranscodingSegmentCleaner"),
			t.serverConfigurationManager,
			t.fileSystem,
			t.mediaEncoder,
			state.SegmentLength(),
		)
		transcodingJob.TranscodingSegmentCleaner.Start()
	}
}

func (t *TranscodeManager) EnableSegmentCleaning(state streaming.StreamState) bool {
	return (state.InputProtocol == mediaprotocol.File || state.InputProtocol == mediaprotocol.Http) &&
		state.IsInputVideo &&
		state.TranscodingType == mediaencoding.Hls &&
		state.RunTimeTicks != nil &&
		*state.RunTimeTicks >= time.Minute.Nanoseconds()*5/100
}

func (t *TranscodeManager) DeleteHlsPartialStreamFiles(outputFilePath string) error {
	directory := filepath.Dir(outputFilePath)
	if directory == "/" {
		return fmt.Errorf("Path can't be a root directory: %s", outputFilePath)
	}

	name := filepath.Base(outputFilePath)
	name = name[:len(name)-len(filepath.Ext(name))]

	filesToDelete, err := t.fileSystem.GetFilePaths(directory, false)
	if err != nil {
		return err
	}

	var exs []error
	for _, file := range filesToDelete {
		if strings.Contains(filepath.Base(file), name) {
			t.logger.Debugf("Deleting HLS file %s", file)
			err := t.fileSystem.DeleteFile(file)
			if err != nil {
				exs = append(exs, err)
				t.logger.Errorf("Error deleting HLS file %s: %v", file, err)
			}
		}
	}

	if len(exs) > 0 {
		return fmt.Errorf("Error deleting HLS files: %v", exs)
	}

	return nil
}

func (t *TranscodeManager) DeleteProgressivePartialStreamFiles(outputFilePath string) error {
	if _, err := os.Stat(outputFilePath); err == nil {
		err = os.Remove(outputFilePath)
		if err != nil {
			t.logger.Errorf("Error deleteing file %s: %v", outputFilePath, err)
			return err
		}
	}

	return nil
}

func (t *TranscodeManager) DeleteEncodedMediaCache() {
	path := t.serverConfigurationManager.GetTranscodePath()
	if !t.fileSystem.DirectoryExists(path) {
		return
	}

	filePaths, err := t.fileSystem.GetFilePaths(path, true)
	if err != nil {
		t.logger.Errorf("Error getting file paths for encoded media cache: %v", err)
		return
	}

	for _, file := range filePaths {
		err := t.fileSystem.DeleteFile(file)
		if err != nil {
			t.logger.Errorf("Error deleting encoded media cache file %s: %v", path, err)
		}
	}
}

package encoder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	//	"strconv"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"files/pkg/media/mediabrowser/controller/configuration"
	"files/pkg/media/mediabrowser/controller/mediaencoding"
	"files/pkg/media/mediabrowser/mediaencoding/probing"
	"files/pkg/media/mediabrowser/model/dlna"
	"files/pkg/media/mediabrowser/model/dto"
	"files/pkg/media/mediabrowser/model/entities"
	"files/pkg/media/mediabrowser/model/mediainfo"
	"files/pkg/media/mediabrowser/model/mediainfo/mediaprotocol"
	"files/pkg/media/utils"
	"files/pkg/media/utils/version"

	"k8s.io/klog/v2"
)

var vulkanExternalMemoryDmaBufExts = []string{
	"VK_KHR_external_memory_fd",
	"VK_EXT_external_memory_dma_buf",
	"VK_KHR_external_semaphore_fd",
	"VK_EXT_external_memory_host",
}

type MediaEncoder struct {
	ffmpegVersion                        *version.Version
	ffmpegPath                           *string
	ffprobePath                          *string
	startupOptionFFmpegPath              string
	logger                               *utils.Logger
	encoders                             []string
	decoders                             []string
	hwaccels                             []string
	filters                              []string
	filtersWithOption                    map[int]bool
	bitStreamFiltersWithOption           map[mediaencoding.BitStreamFilterOptionType]bool
	isPkeyPauseSupported                 bool
	isLowPriorityHwDecodeSupported       bool
	proberSupportsFirstVideoFrame        bool
	isVaapiDeviceAmd                     bool
	isVaapiDeviceInteliHD                bool
	isVaapiDeviceInteli965               bool
	isVaapiDeviceSupportVulkanDrmInterop bool
	threads                              int
	configurationManager                 configuration.IServerConfigurationManager
}

// func NewMediaEncoder() mediaencoding.IMediaEncoder {
func NewMediaEncoder(logger *utils.Logger, configurationManager configuration.IServerConfigurationManager) *MediaEncoder {
	return &MediaEncoder{
		encoders:             []string{},
		logger:               logger,
		configurationManager: configurationManager,
		isPkeyPauseSupported: true,
	}
}

func (m *MediaEncoder) CanEncodeToAudioCodec(codec string) bool {
	switch strings.ToLower(codec) {
	case "opus":
		codec = "libopus"
	case "mp3":
		codec = "libmp3lame"
	}

	return m.SupportsEncoder(codec)
}

func (m *MediaEncoder) CanEncodeToSubtitleCodec(codec string) bool {
	return false
}

func (m *MediaEncoder) CanExtractSubtitles(codec string) bool {
	return false
}

func (m *MediaEncoder) setAvailableEncoders(list []string) {
	m.encoders = make([]string, len(list))
	copy(m.encoders, list)
}

func (m *MediaEncoder) supportsEncoder(encoder string) bool {
	for _, enc := range m.encoders {
		if strings.EqualFold(enc, encoder) {
			return true
		}
	}
	return false
}

func (m *MediaEncoder) IsPkeyPauseSupported() bool {
	return m.isPkeyPauseSupported
}

func (m *MediaEncoder) IsVaapiDeviceAmd() bool {
	return m.isVaapiDeviceAmd
}

func (m *MediaEncoder) IsVaapiDeviceInteliHD() bool {
	return m.isVaapiDeviceInteliHD
}

func (m *MediaEncoder) IsVaapiDeviceInteli965() bool {
	return m.isVaapiDeviceInteli965
}

func (m *MediaEncoder) IsVaapiDeviceSupportVulkanDrmInterop() bool {
	return m.isVaapiDeviceSupportVulkanDrmInterop
}

func (m *MediaEncoder) EncoderPath() string {
	return *m.ffmpegPath
}

func (m *MediaEncoder) ProbePath() string {
	return *m.ffprobePath
}

func (m *MediaEncoder) EncoderVersion() *version.Version {
	return m.ffmpegVersion
}

func (m *MediaEncoder) validatePath(path string) bool {
	if path == "" {
		return false
	}

	ev := NewEncoderValidator(m.logger, path)
	rc := ev.ValidateVersion()
	if !rc {
		m.logger.Warn("FFmpeg: Failed version check: %s", path)
		return false
	}

	m.ffmpegPath = &path
	return true
}

func (e *MediaEncoder) SetMediaEncoderVersion(validator EncoderValidator) {
	e.ffmpegVersion = validator.GetFFmpegVersion()
}

func (m *MediaEncoder) SupportsEncoder(encoder string) bool {
	for _, e := range m.encoders {
		if strings.EqualFold(e, encoder) {
			return true
		}
	}
	return false
}

func (m *MediaEncoder) SupportsDecoder(decoder string) bool {
	for _, d := range m.decoders {
		if strings.EqualFold(d, decoder) {
			return true
		}
	}
	return false
}

func (m *MediaEncoder) SupportsHwaccel(hwaccel string) bool {
	for _, h := range m.hwaccels {
		if strings.EqualFold(h, hwaccel) {
			return true
		}
	}
	return false
}

func (m *MediaEncoder) SupportsFilter(filter string) bool {
	for _, f := range m.filters {
		if strings.EqualFold(f, filter) {
			return true
		}
	}
	return false
}

func (m *MediaEncoder) SupportsFilterWithOption(option mediaencoding.FilterOptionType) bool {
	if val, ok := m.filtersWithOption[int(option)]; ok {
		return val
	}
	return false
}

func (m *MediaEncoder) GetTimeParameter(ticks int64) string {
	duration := time.Duration(ticks * 100)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", int(duration.Hours()), int(duration.Minutes())%60, int(duration.Seconds())%60, int(duration.Milliseconds())%1000)
}

func (m *MediaEncoder) ConvertImage(inputPath, outputPath string) error {
	return nil
}

func (m *MediaEncoder) EscapeSubtitleFilterPath(path string) string {
	return ""
}

func (m *MediaEncoder) SetAvailableEncoders(list []string) {
	m.encoders = make([]string, len(list))
	copy(m.encoders, list)
}

func (m *MediaEncoder) SetAvailableDecoders(list []string) {
	m.decoders = make([]string, len(list))
	copy(m.decoders, list)
}

func (m *MediaEncoder) SetAvailableHwaccels(list []string) {
	m.hwaccels = make([]string, len(list))
	copy(m.hwaccels, list)
}

func (m *MediaEncoder) SetAvailableFilters(list []string) {
	m.filters = make([]string, len(list))
	copy(m.filters, list)
}

func (m *MediaEncoder) SetAvailableFiltersWithOption(dict map[int]bool) {
	m.filtersWithOption = make(map[int]bool, len(dict))
	for k, v := range dict {
		m.filtersWithOption[k] = v
	}
}

func (m *MediaEncoder) ffprobePathRegex() *regexp.Regexp {
	pattern := `[^/\\]+?(\.[^/\\\n.]+)?$`
	return regexp.MustCompile(pattern)
}

func (m *MediaEncoder) SetFFmpegPath() bool {
	// 1) Check if the --ffmpeg CLI switch has been given
	ffmpegPath := m.startupOptionFFmpegPath
	ffmpegPathSetMethodText := "command line or environment variable"
	if ffmpegPath == "" {
		// 2) Custom path stored in config/encoding xml file under tag <EncoderAppPath> should be used as a fallback
		klog.Infoln("GetEncodingOptions: ", m.configurationManager.GetEncodingOptions())
		klog.Infoln("EncoderAppPath", m.configurationManager.GetEncodingOptions().EncoderAppPath)
		ffmpegPath = m.configurationManager.GetEncodingOptions().EncoderAppPath
		ffmpegPathSetMethodText = "encoding.xml config file"
		if ffmpegPath == "" {
			// 3) Check "ffmpeg"
			ffmpegPath = "ffmpeg"
			ffmpegPathSetMethodText = "system $PATH"
		}
	}

	if !m.validatePath(ffmpegPath) {
		m.ffmpegPath = nil
		m.logger.Infof("FFmpeg: Path set by %s is invalid", ffmpegPathSetMethodText)
		return false
	}

	// Write the FFmpeg path to the config/encoding.xml file as <EncoderAppPathDisplay> so it appears in UI
	options := m.configurationManager.GetEncodingOptions()
	klog.Infof("1819 %+v", options)
	if m.ffmpegPath != nil {
		options.EncoderAppPathDisplay = *m.ffmpegPath
	} else {
		options.EncoderAppPathDisplay = ""
	}
	m.configurationManager.SaveConfigurationByKey("encoding", options)

	// Only if mpeg path is set, try and set path to probe
	if m.ffmpegPath != nil {
		// Determine a probe path from the mpeg path
		klog.Infof("ffmpeg path: %v\n", *m.ffmpegPath)
		ffprobePath := m.ffprobePathRegex().ReplaceAllString(*m.ffmpegPath, "ffprobe$1")
		m.ffprobePath = &ffprobePath

		// Interrogate to understand what coders are supported
		validator := NewEncoderValidator(m.logger, *m.ffmpegPath)

		m.SetAvailableDecoders(validator.GetDecoders())
		m.SetAvailableEncoders(validator.GetEncoders())
		m.SetAvailableFilters(validator.GetFilters())
		klog.Infoln("2025-x")
		m.SetAvailableFiltersWithOption(validator.GetFiltersWithOption())
		// to do
		//		m.SetAvailableBitStreamFiltersWithOption(validator.GetBitStreamFiltersWithOption())
		m.SetAvailableHwaccels(validator.GetHwaccels())
		m.SetMediaEncoderVersion(*validator)

		m.threads = mediaencoding.GetNumberOfThreads(nil, *options, nil)

		m.isPkeyPauseSupported = validator.CheckSupportedRuntimeKey("p      pause transcoding", *m.ffmpegVersion)
		klog.Infof("isPkeyPauseSupported: %v\n", m.isPkeyPauseSupported)
		m.isLowPriorityHwDecodeSupported = validator.CheckSupportedHwaccelFlag("low_priority")
		klog.Infof("isLowPriorityHwDecodeSupported: %v\n", m.isLowPriorityHwDecodeSupported)
		m.proberSupportsFirstVideoFrame = validator.CheckSupportedProberOption("only_first_vframe", *m.ffprobePath)
		klog.Infof("proberSupportsFirstVideoFrame: %v\n", m.proberSupportsFirstVideoFrame)

		m.logger.Infof("1819=aaaaaaaaaaaaaaaaaa %v %v %v %v %v", options.VaapiDevice, runtime.GOOS == "linux", m.SupportsHwaccel("vaapi"), options.VaapiDevice != "", options.HardwareAccelerationType == entities.HardwareAccelerationType_VAAPI)
		// Check the Vaapi device vendor
		if runtime.GOOS == "linux" &&
			m.SupportsHwaccel("vaapi") &&
			options.VaapiDevice != "" &&
			options.HardwareAccelerationType == entities.HardwareAccelerationType_VAAPI {
			klog.Infoln("VaapiDevice: ", options.VaapiDevice)
			m.isVaapiDeviceAmd = validator.CheckVaapiDeviceByDriverName("Mesa Gallium driver", options.VaapiDevice)
			m.isVaapiDeviceInteliHD = validator.CheckVaapiDeviceByDriverName("Intel iHD driver", options.VaapiDevice)
			m.isVaapiDeviceInteli965 = validator.CheckVaapiDeviceByDriverName("Intel i965 driver", options.VaapiDevice)
			m.isVaapiDeviceSupportVulkanDrmInterop = validator.CheckVulkanDrmDeviceByExtensionName(options.VaapiDevice, vulkanExternalMemoryDmaBufExts)

			if m.isVaapiDeviceAmd {
				//m.logger.LogInfo("VAAPI device %s is AMD GPU", options.VaapiDevice)
				m.logger.Infof("VAAPI device %s is AMD GPU", options.VaapiDevice)
			} else if m.isVaapiDeviceInteliHD {
				//m.logger.LogInfo("VAAPI device %s is Intel GPU (iHD)", options.VaapiDevice)
				m.logger.Infof("VAAPI device %s is Intel GPU (iHD)", options.VaapiDevice)
			} else if m.isVaapiDeviceInteli965 {
				//m.logger.LogInfo("VAAPI device %s is Intel GPU (i965)", options.VaapiDevice)
				m.logger.Infof("VAAPI device %s is Intel GPU (i965)", options.VaapiDevice)
			}

			if m.isVaapiDeviceSupportVulkanDrmInterop {
				//m.logger.LogInfo("VAAPI device %s supports Vulkan DRM interop", options.VaapiDevice)
				m.logger.Infof("VAAPI device %s supports Vulkan DRM interop", options.VaapiDevice)
			}
		}
	}

	//m.logger.LogInfo("FFmpeg: %s", m.ffmpegPath)
	m.logger.Infof("FFmpeg: %s", *m.ffmpegPath)
	return !(m.ffmpegPath == nil || *m.ffmpegPath == "")
}

func (m *MediaEncoder) UpdateEncoderPath(path, pathType string) {
}

func (m *MediaEncoder) GetPrimaryPlaylistVobFiles(path string, titleNumber *uint) []string {
	/*
	   // Eliminate menus and intros by omitting VIDEO_TS.VOB and all subsequent title .vob files ending with _0.VOB
	   allVobs := listAllVobFiles(path)
	   filteredVobs := filterVobFiles(allVobs)

	   if titleNumber != nil {
	       prefix := fmt.Sprintf("VTS_%02d_", *titleNumber)
	       vobs := filterVobsByPrefix(filteredVobs, prefix)
	       if len(vobs) > 0 {
	           return vobs
	       }
	       log.Warningf("Could not determine .vob files for title %d of %s.", *titleNumber, path)
	   }

	   // Check for multiple big titles (> 900 MB)
	   titles := findBigTitles(filteredVobs)
	   if len(titles) == 0 {
	       titles = []string{getTitleFromVobFile(filteredVobs[0])}
	   }

	   // Aggregate all .vob files of the titles
	   return aggregateVobFiles(filteredVobs, titles)
	*/
	return []string{}
}

/*
func listAllVobFiles(path string) []os.DirEntry {
}

func filterVobFiles(files []os.DirEntry) []os.DirEntry {
    files, err := os.ReadDir(path)
    if err != nil {
        // Handle the error
        return nil
    }

    var allVobs []os.DirEntry
    for _, file := range files {
        if filepath.Ext(file.Name()) == ".VOB" &&
           !strings.EqualFold(file.Name(), "VIDEO_TS.VOB") &&
           !strings.HasSuffix(file.Name(), "_0.VOB") {
            allVobs = append(allVobs, file)
        }
    }

    sort.Slice(allVobs, func(i, j int) bool {
        return allVobs[i].Name() < allVobs[j].Name()
    })

    return allVobs
}

func filterVobsByPrefix(files []os.DirEntry, prefix string) []string {
    var vobs []os.DirEntry
    for _, file := range allVobs {
        if strings.HasPrefixFold(file.Name(), prefix) {
            vobs = append(vobs, file)
        }
    }
    return vobs
}

func findBigTitles(files []os.DirEntry) []string {
    const minSize = 900 * 1024 * 1024 // 900 MB

    var titles = make(map[string]bool)
    for _, vob := range allVobs {
        if vob.Size() >= minSize {
            title := strings.Split(fileSystem.GetFileNameWithoutExtension(vob.Name()), "_")[1]
            titles[title] = true
        }
    }

    var bigTitles []string
    for title := range titles {
        bigTitles = append(bigTitles, title)
    }
    return bigTitles
}

func getTitleFromVobFile(file os.DirEntry) string {
    if len(allVobs) > 0 {
        title := strings.Split(fileSystem.GetFileNameWithoutExtension(allVobs[0].Name()), "_")[1]
        titles[title] = true
    }
}

func aggregateVobFiles(files []os.DirEntry, titles []string) []string {
    var matchingVobs []string
    for _, vob := range allVobs {
        title := strings.Split(fileSystem.GetFileNameWithoutExtension(vob.Name()), "_")[1]
        if titles[title] {
            matchingVobs = append(matchingVobs, vob.Name())
        }
    }
    sort.Strings(matchingVobs)
    return matchingVobs
}
*/

func (m *MediaEncoder) GetPrimaryPlaylistM2tsFiles(path string) []string {
	return []string{}
}

func (m *MediaEncoder) GetInputPathArgument(state mediaencoding.EncodingJobInfo) string {
	klog.Infoln("state mediapath:", state.MediaPath)
	return m.GetInputPathArgument2(state.MediaPath, *state.MediaSource)
}

func (m *MediaEncoder) GetInputPathArgument2(path string, mediaSource dto.MediaSourceInfo) string {
	if mediaSource.VideoType == nil {
		return ""
	}

	switch *mediaSource.VideoType {
	case entities.Dvd:
		return m.GetInputArgumentArray(m.GetPrimaryPlaylistVobFiles(path, nil), mediaSource)
	case entities.BluRay:
		return m.GetInputArgumentArray(m.GetPrimaryPlaylistM2tsFiles(path), mediaSource)
	default:
		//return m.GetInputArgument([]string{path}, mediaSource)
		return m.GetInputArgument(path, mediaSource)
	}
}

func (m *MediaEncoder) GetInputArgumentArray(inputFiles []string, mediaSource dto.MediaSourceInfo) string {
	return GetInputArgumentArray("file", inputFiles, mediaSource.Protocol)
}

func (m *MediaEncoder) GetInputArgument(inputFile string, mediaSource dto.MediaSourceInfo) string {
	prefix := "file"
	if mediaSource.IsoType != nil && *mediaSource.IsoType == entities.BluRay2 {
		prefix = "bluray"
	}
	return GetInputArgumentArray(prefix, []string{inputFile}, mediaSource.Protocol)
}

func (m *MediaEncoder) GetExternalSubtitleInputArgument(inputFile string) string {
	const Prefix = "file"
	return GetInputArgumentArray(Prefix, []string{inputFile}, mediaprotocol.File)
}

func (m *MediaEncoder) GenerateConcatConfig(source dto.MediaSourceInfo, concatFilePath string) {
}

/*

func (m *MediaEncoder) GenerateConcatConfig(source MediaSourceInfo, concatFilePath string) {
    // Get all playable files
    var files []string
    videoType := source.VideoType
    if videoType == VideoType_Dvd {
        files = GetPrimaryPlaylistVobFiles(source.Path, nil)
    } else if videoType == VideoType_BluRay {
        files = GetPrimaryPlaylistM2tsFiles(source.Path)
    } else {
        return
    }

    // Generate concat configuration entries for each file and write to file
    f, err := os.Create(concatFilePath)
    if err != nil {
        // Handle the error
        return
    }
    defer f.Close()

    for _, path := range files {
        mediaInfoResult, err := GetMediaInfo(MediaInfoRequest{
            MediaType:  DlnaProfileType_Video,
            MediaSource: &MediaSourceInfo{
                Path:     path,
                Protocol: MediaProtocol_File,
                VideoType: videoType,
            },
        }, context.Background())
        if err != nil {
            // Handle the error
            return
        }

        duration := float64(mediaInfoResult.RunTimeTicks.Value) / float64(time.Second)

        // Add file path stanza to concat configuration
        fmt.Fprintf(f, "file '%s'\n", path)

        // Add duration stanza to concat configuration
        fmt.Fprintf(f, "duration %f\n", duration)
    }
}

func GetPrimaryPlaylistVobFiles(path string, _ interface{}) []string {
    // Implement the logic to get the primary playlist VOB files
    return []string{"file1.vob", "file2.vob", "file3.vob"}
}

func GetPrimaryPlaylistM2tsFiles(path string) []string {
    // Implement the logic to get the primary playlist M2TS files
    return []string{"file1.m2ts", "file2.m2ts", "file3.m2ts"}
}
*/

func (m *MediaEncoder) GetMediaInfo(request *mediaencoding.MediaInfoRequest, ctx context.Context, headers string) (*mediainfo.MediaInfo, error) {
	extractChapters := request.MediaType == dlna.Video && request.ExtractChapters
	extraArgs := m.GetExtraArguments(request)

	return m.GetMediaInfoInternal(
		m.GetInputArgument(request.MediaSource.Path, *request.MediaSource),
		request.MediaSource.Path,
		request.MediaSource.Protocol,
		extractChapters,
		extraArgs,
		request.MediaType == dlna.Audio,
		request.MediaSource.VideoType,
		ctx,
		headers,
	)
}

func (m *MediaEncoder) GetMediaInfoInternal(
	inputPath string,
	primaryPath string,
	protocol mediaprotocol.MediaProtocol,
	extractChapters bool,
	probeSizeArgument string,
	isAudio bool,
	videoType *entities.VideoType,
	ctx context.Context,
	headers string,
) (*mediainfo.MediaInfo, error) {
	args := "-v warning -print_format json -show_streams -show_format"
	if extractChapters {
		args += " -show_chapters"
	}

	if protocol == mediaprotocol.Http {
		if headers != "" {
			args += ` -headers "` + headers + `"`
		}
	}

	args = fmt.Sprintf(`%s -i %s -threads %d`, args, inputPath, m.threads)
	args = strings.TrimSpace(args)
	/*
		args := []string{
			"-v" ,
			"warning",
			"-print_format",
			"json",
			"-show_streams",
			"-show_format",
		}
		if  extractChapters {
			args = append(args, "-show_chapters")
		}
		args = append(args, []string{
			"-i",
			inputPath,
			"-threads",
			fmt.Sprintf("%d", m.threads),
		}...)
	*/

	//cmd := exec.CommandContext(ctx, *m.ffprobePath, strings.Split(args, " ")...)
	//cmd := exec.CommandContext(ctx, *m.ffprobePath, args...)
	if m.ffprobePath == nil {
		klog.Infoln("probe.........................")
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", *m.ffprobePath+" "+args)

	cmd.SysProcAttr = &syscall.SysProcAttr{
		//		CreationFlags: 0,
		//		HideWindow:    true,
	}
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	m.logger.Infof("Starting %s with args %s\n", *m.ffprobePath, args)

	if err := cmd.Start(); err != nil {
		klog.Infoln("Start Errrrrrrrrrrrrr")
		klog.Infoln(err)
		return nil, err
	}

	var memoryBuffer bytes.Buffer
	if _, err := io.Copy(&memoryBuffer, stdout); err != nil {
		cmd.Process.Kill()
		klog.Infoln("Copy Errrrrrrrrrrrrr")
		klog.Infoln(err)
		return nil, err
	}

	if err := cmd.Wait(); err != nil {
		klog.Infoln("Wait Errrrrrrrrrrrrr")
		klog.Infoln(err)
		return nil, err
	}

	klog.Infoln(string(memoryBuffer.Bytes()))
	var result probing.InternalMediaInfoResult
	if err := json.Unmarshal(memoryBuffer.Bytes(), &result); err != nil {
		return nil, err
	}
	for _, stream := range result.Streams {
		klog.Infof("---- > %+v\n", stream)
	}

	klog.Infof("---- > %+v\n", result.Format)
	klog.Infof("---- > %+v\n", result.Chapters)

	if /*result == nil || */ result.Streams == nil && result.Format == nil {
		return nil, fmt.Errorf("ffprobe failed - streams and format are both null")
	}

	if result.Streams != nil {
		for _, stream := range result.Streams {
			if stream.DisplayAspectRatio == "0:1" {
				stream.DisplayAspectRatio = ""
			}
			if stream.SampleAspectRatio == "0:1" {
				stream.SampleAspectRatio = ""
			}
		}
	}

	return probing.NewProbeResultNormalizer(m.logger).GetMediaInfo(&result, videoType, isAudio, primaryPath, protocol), nil
}

func (m *MediaEncoder) GetExtraArguments(request *mediaencoding.MediaInfoRequest) string {
	ffmpegAnalyzeDuration := "200M" //m.config.GetFFmpegAnalyzeDuration()
	ffmpegProbeSize := "1G"         //m.config.GetFFmpegProbeSize()

	var analyzeDuration, extraArgs string

	if request.MediaSource.AnalyzeDurationMs != nil && *request.MediaSource.AnalyzeDurationMs > 0 {
		analyzeDuration = fmt.Sprintf("-analyzeduration %d", *request.MediaSource.AnalyzeDurationMs*1000)
	} else if ffmpegAnalyzeDuration != "" {
		analyzeDuration = "-analyzeduration " + ffmpegAnalyzeDuration
	}

	if analyzeDuration != "" {
		extraArgs = analyzeDuration
	}

	if ffmpegProbeSize != "" {
		extraArgs += " -probesize " + ffmpegProbeSize
	}

	if userAgent, ok := request.MediaSource.RequiredHttpHeaders["User-Agent"]; ok {
		extraArgs += fmt.Sprintf(" -user_agent \"%s\"", userAgent)
	}

	if request.MediaSource.Protocol == mediaprotocol.Rtsp {
		extraArgs += " -rtsp_transport tcp+udp -rtsp_flags prefer_tcp"
	}

	return extraArgs
}

func (m *MediaEncoder) SupportsBitStreamFilterWithOption(option mediaencoding.BitStreamFilterOptionType) bool {
	val, ok := m.bitStreamFiltersWithOption[option]
	return ok && val
}

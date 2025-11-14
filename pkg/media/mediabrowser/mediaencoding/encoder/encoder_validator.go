package encoder

import (
	"bufio"
	"bytes"
	"log"
	"os/exec"

	//	"errors"
	"fmt"
	//	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"files/pkg/media/utils"
	"files/pkg/media/utils/version"
)

var requiredDecoders = []string{
	"h264",
	"hevc",
	"vp8",
	"libvpx",
	"vp9",
	"libvpx-vp9",
	"av1",
	"libdav1d",
	"mpeg2video",
	"mpeg4",
	"msmpeg4",
	"dca",
	"ac3",
	"ac4",
	"aac",
	"mp3",
	"flac",
	"truehd",
	"h264_qsv",
	"hevc_qsv",
	"mpeg2_qsv",
	"vc1_qsv",
	"vp8_qsv",
	"vp9_qsv",
	"av1_qsv",
	"h264_cuvid",
	"hevc_cuvid",
	"mpeg2_cuvid",
	"vc1_cuvid",
	"mpeg4_cuvid",
	"vp8_cuvid",
	"vp9_cuvid",
	"av1_cuvid",
	"h264_rkmpp",
	"hevc_rkmpp",
	"mpeg1_rkmpp",
	"mpeg2_rkmpp",
	"mpeg4_rkmpp",
	"vp8_rkmpp",
	"vp9_rkmpp",
	"av1_rkmpp",
}

var requiredEncoders = []string{
	"libx264",
	"libx265",
	"libsvtav1",
	"aac",
	"aac_at",
	"libfdk_aac",
	"ac3",
	"alac",
	"dca",
	"libmp3lame",
	"libopus",
	"libvorbis",
	"flac",
	"truehd",
	"srt",
	"h264_amf",
	"hevc_amf",
	"av1_amf",
	"h264_qsv",
	"hevc_qsv",
	"mjpeg_qsv",
	"av1_qsv",
	"h264_nvenc",
	"hevc_nvenc",
	"av1_nvenc",
	"h264_vaapi",
	"hevc_vaapi",
	"av1_vaapi",
	"mjpeg_vaapi",
	"h264_v4l2m2m",
	"h264_videotoolbox",
	"hevc_videotoolbox",
	"mjpeg_videotoolbox",
	"h264_rkmpp",
	"hevc_rkmpp",
	"mjpeg_rkmpp",
}

var requiredFilters = []string{
	// sw
	"alphasrc",
	"zscale",
	"tonemapx",
	// qsv
	"scale_qsv",
	"vpp_qsv",
	"deinterlace_qsv",
	"overlay_qsv",
	// cuda
	"scale_cuda",
	"yadif_cuda",
	"bwdif_cuda",
	"tonemap_cuda",
	"overlay_cuda",
	"transpose_cuda",
	"hwupload_cuda",
	// opencl
	"scale_opencl",
	"tonemap_opencl",
	"overlay_opencl",
	"transpose_opencl",
	"yadif_opencl",
	"bwdif_opencl",
	// vaapi
	"scale_vaapi",
	"deinterlace_vaapi",
	"tonemap_vaapi",
	"procamp_vaapi",
	"overlay_vaapi",
	"transpose_vaapi",
	"hwupload_vaapi",
	// vulkan
	"libplacebo",
	"scale_vulkan",
	"overlay_vulkan",
	"transpose_vulkan",
	"flip_vulkan",
	// videotoolbox
	"yadif_videotoolbox",
	"bwdif_videotoolbox",
	"scale_vt",
	"transpose_vt",
	"overlay_videotoolbox",
	"tonemap_videotoolbox",
	// rkrga
	"scale_rkrga",
	"vpp_rkrga",
	"overlay_rkrga",
}

var filterOptionsDict = map[int][]string{
	0: {"scale_cuda", "format"},
	1: {"tonemap_cuda", "GPU accelerated HDR to SDR tonemapping"},
	2: {"tonemap_opencl", "bt2390"},
	3: {"overlay_opencl", "Action to take when encountering EOF from secondary input"},
	4: {"overlay_vaapi", "Action to take when encountering EOF from secondary input"},
	5: {"overlay_vulkan", "Action to take when encountering EOF from secondary input"},
	6: {"transpose_opencl", "rotate by half-turn"},
	7: {"overlay_opencl", "alpha_format"},
	8: {"overlay_cuda", "alpha_format"},
}

var ffmpegMinimumLibraryVersions = map[string]*version.Version{
	"libavutil":     {Major: 56, Minor: 70},
	"libavcodec":    {Major: 58, Minor: 134},
	"libavformat":   {Major: 58, Minor: 76},
	"libavdevice":   {Major: 58, Minor: 13},
	"libavfilter":   {Major: 7, Minor: 110},
	"libswscale":    {Major: 5, Minor: 9},
	"libswresample": {Major: 3, Minor: 9},
	"libpostproc":   {Major: 55, Minor: 9},
}

var minFFmpegMultiThreadedCli = version.Version{Major: 7, Minor: 0}

func FfmpegVersionRegex() *regexp.Regexp {
	return regexp.MustCompile(`^ffmpeg version n?((?:[0-9]+\.?)+)`)
}

func LibraryRegex() *regexp.Regexp {
	return regexp.MustCompile(`((?<name>lib\w+)\s+(?<major>[0-9]+)\.\s*(?<minor>[0-9]+))`)
}

type Codec int

const (
	Encoder Codec = iota
	Decoder
)

func NewEncoderValidator(logger *utils.Logger, encoderPath string) *EncoderValidator {
	return &EncoderValidator{
		logger:      logger,
		encoderPath: encoderPath,
	}
}

type EncoderValidator struct {
	encoderPath                  string
	logger                       *utils.Logger
	minVersion                   version.Version
	maxVersion                   *version.Version
	FFmpegPath                   string
	FFmpegMinimumLibraryVersions map[string]version.Version
	// requiredEncoders             []string
	// requiredDecoders             []string
	// requiredFilters              []string
	// filterOptionsDict            map[int][]string
}

type Logger interface {
	LogError(err error, msg string, args ...interface{})
	LogDebug(msg string, args ...interface{})
	LogInformation(msg string, args ...interface{})
	LogWarning(msg string, args ...interface{})
}

func (ev *EncoderValidator) ValidateVersion() bool {
	output, err := ev.getProcessOutput(ev.encoderPath, "-version", false, "")
	if err != nil {
		//ev.logger.LogError(err, "Error validating encoder")
		fmt.Errorf("Error validating encoder %v\n", err)
		return false
	}

	if output == "" {
		//ev.logger.LogError(errors.New("FFmpeg validation: The process returned no result"), "")
		fmt.Errorf("FFmpeg validation: The process returned no result")
		return false
	}

	//ev.logger.LogDebug("ffmpeg output: %s", output)
	ev.logger.Debugf("ffmpeg output: %s", output)

	return ev.validateVersionInternal(output)
}

func (ev *EncoderValidator) validateVersionInternal(versionOutput string) bool {
	/*
		if strings.Contains(strings.ToLower(versionOutput), "libav developers") {
			//ev.logger.LogError(errors.New("FFmpeg validation: avconv instead of ffmpeg is not supported"), "")
			ev.logger.Error("FFmpeg validation: avconv instead of ffmpeg is not supported")
			return false
		}

		version := ev.getFFmpegVersionInternal(versionOutput)

		//ev.logger.LogInformation("Found ffmpeg version %v", version)
		ev.logger.Infof("Found ffmpeg version %v", version)

		if version == nil {
			if ev.maxVersion != nil { // Version is unknown
				if ev.minVersion.Equal(*ev.maxVersion) {
					ev.logger.Warnf("FFmpeg validation: We recommend version %v", ev.minVersion)
				} else {
					ev.logger.Warnf("FFmpeg validation: We recommend a minimum of %v and maximum of %v", ev.minVersion, ev.maxVersion)
				}
			} else {
				ev.logger.Warnf("FFmpeg validation: We recommend minimum version %v", ev.minVersion)
			}
			return false
		}

		if version.LessThan(ev.minVersion) { // Version is below what we recommend
			//ev.logger.LogWarning("FFmpeg validation: The minimum recommended version is %v", ev.minVersion)
			ev.logger.Warnf("FFmpeg validation: The minimum recommended version is %v", ev.minVersion)
			return false
		}

		if ev.maxVersion != nil && version.GreaterThan(*ev.maxVersion) { // Version is above what we recommend
			ev.logger.LogWarning("FFmpeg validation: The maximum recommended version is %v", ev.maxVersion)
			return false
		}
	*/
	return true
}

func (ev *EncoderValidator) GetDecoders() []string {
	return ev.getCodecs(Decoder)
}

func (ev *EncoderValidator) GetEncoders() []string {
	return ev.getCodecs(Encoder)
}

func (ev *EncoderValidator) GetHwaccels() []string {
	return ev.getHwaccelTypes()
}

func (ev *EncoderValidator) GetFilters() []string {
	return ev.getFFmpegFilters()
}

func (ev *EncoderValidator) GetFiltersWithOption() map[int]bool {
	return ev.getFFmpegFiltersWithOption()
}

func (ev *EncoderValidator) GetFFmpegVersion() *version.Version {
	output, err := ev.getProcessOutput(ev.encoderPath, "-version", false, "")
	if err != nil {
		//ev.logger.LogError(err, "Error validating encoder")
		ev.logger.Errorf("Error validating encoder %v", err)
		return nil
	}

	if output == "" {
		//ev.logger.LogError(errors.New("FFmpeg validation: The process returned no result"), "")
		fmt.Println("FFmpeg validation: The process returned no result")
		return nil
	}

	//ev.logger.LogDebug("ffmpeg output: %s", output)
	ev.logger.Debugf("ffmpeg output: %s", output)

	return ev.getFFmpegVersionInternal(output)
}

func (ev *EncoderValidator) getFFmpegVersionInternal(versionOutput string) *version.Version {
	// Implement logic to parse the version output and return a Version struct
	// Example:
	versionRegex := regexp.MustCompile(`version\s+(\d+)\.(\d+)\.(\d+)`)
	matches := versionRegex.FindStringSubmatch(versionOutput)
	if len(matches) != 4 {
		return nil
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	return &version.Version{Major: major, Minor: minor, Patch: patch}
}

func (ev *EncoderValidator) getCodecs(codec Codec) []string {
	codecStr := "encoders"
	if codec == Decoder {
		codecStr = "decoders"
	}

	output, err := ev.getProcessOutput(ev.encoderPath, "-"+codecStr, false, "")
	if err != nil {
		//_logger.WithError(err).Errorf("Error detecting available %s", codecStr)
		ev.logger.Errorf("Error detecting available %s %v", codecStr, err)
		return nil
	}

	if strings.TrimSpace(output) == "" {
		return nil
	}

	var required []string
	if codec == Encoder {
		required = requiredEncoders
	} else {
		required = requiredDecoders
	}

	found := codecRegex().FindAllStringSubmatch(output, -1)
	var result []string
	for _, match := range found {
		codec := match[1]
		if contains(required, codec) {
			result = append(result, codec)
		}
	}
	ev.logger.Infof("Available %s: %v", codecStr, result)
	return result
}

func (ev *EncoderValidator) getHwaccelTypes() []string {
	output, err := ev.getProcessOutput(ev.encoderPath, "-hwaccels", false, "")
	if err != nil {
		//_logger.WithError(err).Error("Error detecting available hwaccel types")
		ev.logger.Error("Error detecting available hwaccel types")
		return nil
	}

	if strings.TrimSpace(output) == "" {
		return nil
	}

	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return nil
	}

	found := make(map[string]struct{})
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line != "" {
			found[line] = struct{}{}
		}
	}

	var result []string
	for hwaccel := range found {
		result = append(result, hwaccel)
	}

	ev.logger.Infof("Available hwaccel types: %v", result)
	return result
}

func (ev *EncoderValidator) getFFmpegFilters() []string {
	output, err := ev.getProcessOutput(ev.encoderPath, "-filters", false, "")
	if err != nil {
		ev.logger.Error("Error detecting available filters")
		return nil
	}

	if strings.TrimSpace(output) == "" {
		return nil
	}

	found := filterRegex().FindAllStringSubmatch(output, -1)
	var result []string
	for _, match := range found {
		filter := match[1]
		if contains(requiredFilters, filter) {
			result = append(result, filter)
		}
	}

	ev.logger.Infof("Available filters: %v", result)
	return result
}

func (ev *EncoderValidator) getFFmpegFiltersWithOption() map[int]bool {
	result := make(map[int]bool)
	for i, filterOption := range filterOptionsDict {
		if len(filterOption) == 2 {
			result[i] = ev.CheckFilterWithOption(filterOption[0], filterOption[1])
		}
	}
	return result
}

func (ev *EncoderValidator) GetFFmpegVersionInternal(output string) *version.Version {
	/*
		// For pre-built binaries the FFmpeg version should be mentioned at the very start of the output
		match := ev.FFmpegVersionRegex().FindStringSubmatch(output)
		if len(match) > 1 {
			if ver, err := strconv.Atoi(match[1]); err == nil {
				return &version.Version{Major: ver, Minor: 0}
			}
		}

		versionMap := ev.GetFFmpegLibraryVersions(output)

		allVersionsValidated := true

		for _, minimumVersion := range ev.FFmpegMinimumLibraryVersions {
			if foundVersion, ok := versionMap[minimumVersion.Key]; ok {
				if foundVersion.Major >= minimumVersion.Value.Major && foundVersion.Minor >= minimumVersion.Value.Minor {
					//ev.Logger.LogInformation("Found %s version %d.%d (%d.%d)", minimumVersion.Key, foundVersion.Major, foundVersion.Minor, minimumVersion.Value.Major, minimumVersion.Value.Minor)
					ev.logger.Infof("Found %s version %d.%d (%d.%d)", minimumVersion.Key, foundVersion.Major, foundVersion.Minor, minimumVersion.Value.Major, minimumVersion.Value.Minor)
				} else {
					//ev.Logger.LogWarning("Found %s version %d.%d lower than recommended version %d.%d", minimumVersion.Key, foundVersion.Major, foundVersion.Minor, minimumVersion.Value.Major, minimumVersion.Value.Minor)
					ev.logger.Warnf("Found %s version %d.%d lower than recommended version %d.%d", minimumVersion.Key, foundVersion.Major, foundVersion.Minor, minimumVersion.Value.Major, minimumVersion.Value.Minor)
					allVersionsValidated = false
				}
			} else {
				//ev.Logger.LogError(fmt.Errorf("version not found"), "%s version not found", minimumVersion.Key)
				fmt.Errorf("%s version not found\n", minimumVersion.Key)
				allVersionsValidated = false
			}
		}

		if allVersionsValidated {
			return &Version{Major: 0, Minor: 0}
		} else {
			return nil
		}
	*/
	return nil
}

func (ev *EncoderValidator) GetFFmpegLibraryVersions(output string) map[string]version.Version {
	versionMap := make(map[string]version.Version)

	for _, match := range LibraryRegex().FindAllStringSubmatch(output, -1) {
		major, _ := strconv.Atoi(match[1])
		minor, _ := strconv.Atoi(match[2])
		versionMap[match[3]] = version.Version{Major: major, Minor: minor}
	}

	return versionMap
}

func (ev *EncoderValidator) CheckVaapiDeviceByDriverName(driverName, renderNodePath string) bool {
	fmt.Println("CheckVaapiDeviceByDriverName:", driverName, renderNodePath)
	if !isLinux() {
		return false
	}

	if driverName == "" || renderNodePath == "" {
		return false
	}

	//output, err := ev.getProcessOutput(ev.encoderPath, "-v verbose -hide_banner -init_hw_device vaapi=va:"+renderNodePath, true, "")
	output, err := ev.getProcessOutput(ev.encoderPath, "-v verbose -hide_banner -init_hw_device vaapi=va:"+renderNodePath+" -f lavfi -i testsrc -frames:v 1 -f null -", true, "")
	if err != nil {
		ev.logger.Errorf("Error detecting the given vaapi render node path %v", err)
		return false
	}

	return strings.Contains(output, driverName)
}

func isLinux() bool {
	return runtime.GOOS == "linux"
}

func (ev *EncoderValidator) CheckVulkanDrmDeviceByExtensionName(renderNodePath string, vulkanExtensions []string) bool {
	if !isLinux() {
		return false
	}

	if renderNodePath == "" {
		return false
	}

	command := "-v verbose -hide_banner -init_hw_device drm=dr:" + renderNodePath + " -init_hw_device vulkan=vk@dr"
	output, err := ev.getProcessOutput(ev.encoderPath, command, true, "")
	if err != nil {
		//		ev.Logger.LogError(err, "Error detecting the given drm render node path")
		ev.logger.Errorf("Error detecting the given drm render node path %v", err)
		return false
	}

	for _, ext := range vulkanExtensions {
		if !strings.Contains(output, ext) {
			return false
		}
	}

	return true
}

func (ev *EncoderValidator) GetHwaccelTypes() []string {
	output, err := ev.getProcessOutput(ev.encoderPath, "-hwaccels", false, "")
	if err != nil {
		//ev.Logger.LogError(err, "Error detecting available hwaccel types")
		ev.logger.Errorf("Error detecting available hwaccel types %v", err)
		return []string{}
	}

	if output == "" {
		return []string{}
	}

	found := []string{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Scan() // Skip the first line
	for scanner.Scan() {
		found = append(found, scanner.Text())
	}
	distinctFilters := make(map[string]struct{})
	for _, filter := range found {
		distinctFilters[filter] = struct{}{}
	}

	var result []string
	for filter := range distinctFilters {
		result = append(result, filter)
	}
	//ev.Logger.logInformation("Available hwaccel types: %v", found)
	ev.logger.Infof("Available hwaccel types: %v", result)
	return result
}

func (ev *EncoderValidator) CheckFilterWithOption(filter, option string) bool {
	if filter == "" || option == "" {
		return false
	}

	output, err := ev.getProcessOutput(ev.encoderPath, "-h filter="+filter, false, "")
	if err != nil {
		//ev.Logger.LogError(err, "Error detecting the given filter")
		ev.logger.Errorf("Error detecting the given filter %v", err)
		return false
	}

	if strings.Contains(output, "Filter "+filter) {
		return strings.Contains(output, option)
	}

	//ev.Logger.LogWarning("Filter: %s with option %s is not available", filter, option)
	ev.logger.Warnf("Filter: %s with option %s is not available", filter, option)
	return false
}

func (ev *EncoderValidator) CheckSupportedRuntimeKey(keyDesc string, ffmpegVersion version.Version) bool {
	if keyDesc == "" {
		return false
	}

	// With multi-threaded cli support, FFmpeg 7 is less sensitive to keyboard input
	duration := 1000 // Default duration
	if version.Compare(ffmpegVersion, minFFmpegMultiThreadedCli) >= 0 {
		duration = 10000
	}
	// to resolve
	// duration = 500

	output, err := ev.getProcessOutput(ev.encoderPath, fmt.Sprintf("-hide_banner -f lavfi -i nullsrc=s=1x1:d=%d -f null -", duration), true, "?")
	if err != nil {
		ev.logger.Errorf("Error checking supported runtime key %v", err)
		return false
	}

	return strings.Contains(output, keyDesc)
}

func (ev *EncoderValidator) getProcessExitCode(path, arguments string) bool {
	// Split arguments string into a slice for exec.Command
	args := strings.Fields(arguments)
	log.Printf("Running %s %s", path, arguments)

	cmd := exec.Command(path, args...)
	cmd.Stdout = nil // Equivalent to CreateNoWindow/UseShellExecute=false
	cmd.Stderr = nil // Suppress output, similar to hidden window style

	err := cmd.Run()
	if err != nil {
		log.Printf("Running %s %s failed with error: %v", path, arguments, err)
		return false
	}
	return cmd.ProcessState.ExitCode() == 0
}

func (ev *EncoderValidator) CheckSupportedHwaccelFlag(flag string) bool {
	if flag == "" {
		return false
	}
	return ev.getProcessExitCode(ev.encoderPath, "-loglevel quiet -hwaccel_flags +"+flag+" -hide_banner -f lavfi -i nullsrc=s=1x1:d=100 -f null -")
}

func (ev *EncoderValidator) CheckSupportedProberOption(option, proberPath string) bool {
	if option == "" {
		return false
	}
	return ev.getProcessExitCode(proberPath, "-loglevel quiet -f lavfi -i nullsrc=s=1x1:d=1 -"+option)
}

func (ev *EncoderValidator) GetCodecs(codec Codec) []string {
	codecstr := "encoders"
	if codec == Decoder {
		codecstr = "decoders"
	}

	output, err := ev.getProcessOutput(ev.encoderPath, "-"+codecstr, true, "")
	if err != nil {
		//ev.Logger.LogError(err, "Error detecting available %s", codecstr)
		ev.logger.Errorf("Error detecting available %s %v", codecstr, err)
		return []string{}
	}

	if output == "" {
		return []string{}
	}

	var required []string
	if codec == Encoder {
		required = requiredEncoders
	} else {
		required = requiredDecoders
	}

	found := []string{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		codec := codecRegex().FindStringSubmatch(scanner.Text())
		if len(codec) > 1 && contains(required, codec[1]) {
			found = append(found, codec[1])
		}
	}

	//ev.Logger.LogInformation("Available %s: %v", codecstr, found)
	ev.logger.Infof("Available %s: %v\n", codecstr, found)
	return found
}

func (ev *EncoderValidator) getProcessOutput(path string, arguments string, readStdErr bool, testKey string) (string, error) {
	// Prepare the command
	cmd := exec.Command(path, strings.Split(arguments, " ")...)

	// Create buffers for output and error
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	if readStdErr {
		cmd.Stderr = &stderr
	}

	// If testKey is provided, set up standard input
	if testKey != "" {
		stdin := bytes.NewBufferString(testKey)
		cmd.Stdin = stdin
	}

	log.Printf("Running %s %v", path, arguments)

	// Start the command
	err := cmd.Start()
	if err != nil {
		return "", err
	}

	// Wait for the command to finish
	err = cmd.Wait()
	if err != nil {
		return "", err
	}

	// Return the appropriate output
	if readStdErr {
		return stderr.String(), nil
	}

	return stdout.String(), nil
}

func contains(slice []string, target string) bool {
	for _, s := range slice {
		if s == target {
			return true
		}
	}
	return false
}

func codecRegex() *regexp.Regexp {
	pattern := `\s\S{6}\s(?<codec>[\w|-]+)\s+.+`
	return regexp.MustCompile(pattern)
}

func filterRegex() *regexp.Regexp {
	pattern := `\s\S{3}\s(?<filter>[\w|-]+)\s+.+`
	return regexp.MustCompile(pattern)
}

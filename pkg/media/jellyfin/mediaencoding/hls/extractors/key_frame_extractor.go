package extractors

import (
	"bufio"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"files/pkg/media/jellyfin/mediaencoding/keyframes"
)

type IKeyframeExtractor interface {
	TryExtractKeyframes(filePath string) (keyframes.KeyframeData, bool)
}

type FfProbeKeyframeExtractor struct {
}

func (d *FfProbeKeyframeExtractor) TryExtractKeyframes(filePath string) (keyframes.KeyframeData, bool) {
	//	args := []string{"-fflags", "+genpts", "-select_streams", "v", "-skip_frame", "nokey",
	//		"-show_entries", "format=duration", "-show_entries", "stream=duration", "-show_entries", "packet=pts_time,flags",
	//		"-of", "csv=p=1"}
	command := "-fflags +genpts -v error -skip_frame nokey -show_entries format=duration -show_entries stream=duration -show_entries packet=pts_time,flags -select_streams v -of csv"
	args := strings.Split(command, " ")

	ffmpegCmd := exec.Command("ffprobe", append(args, filePath)...)

	outputPipe, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		fmt.Println("Error creating output pipe:", err)
		return keyframes.KeyframeData{}, false
	}

	err = ffmpegCmd.Start()
	if err != nil {
		fmt.Println("Error starting FFmpeg command:", err)
		return keyframes.KeyframeData{}, false
	}

	fmt.Println("Waiting for FFmpeg command to finish...")
	keyframes := keyframes.KeyframeData{KeyframeTicks: []int64{}}
	scanner := bufio.NewScanner(outputPipe)
	for scanner.Scan() {
		line := string(scanner.Bytes())
		// example：packet,5.000000,K_
		fmt.Println(string(line))

		firstComma := strings.Index(line, ",")
		lineType := line[:firstComma]
		rest := line[firstComma+1:]

		if strings.EqualFold(lineType, "packet") {
			// Split time and flags from the packet line. Example line: packet,7169.079000,K_
			secondComma := strings.Index(rest, ",")
			ptsTime := rest[:secondComma]
			flags := rest[secondComma+1:]
			if strings.HasPrefix(flags, "K_") {
				if keyframe, err := strconv.ParseFloat(ptsTime, 64); err == nil {
					// Have to manually convert to ticks to avoid rounding errors
					keyframes.KeyframeTicks = append(keyframes.KeyframeTicks, int64(keyframe*float64(time.Second)/float64(time.Millisecond)))

				}
			}
		} else if strings.EqualFold(lineType, "format") {
			if formatDurationResult, err := strconv.ParseFloat(rest, 10); err == nil {
				keyframes.TotalDuration = int64(formatDurationResult * float64(time.Second))
			}
		}
	}
	return keyframes, true
}

type MatroskaKeyframeExtractor struct {
}

func (d *MatroskaKeyframeExtractor) TryExtractKeyframes(filePath string) (keyframes.KeyframeData, bool) {
	return keyframes.KeyframeData{}, true
}

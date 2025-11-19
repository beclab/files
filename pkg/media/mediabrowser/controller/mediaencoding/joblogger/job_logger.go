package joblogger

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"files/pkg/media/mediabrowser/controller/streaming"
	// "files/pkg/media/mediabrowser/controller/mediaencoding"
	"k8s.io/klog/v2"
)

/*
type EncodingJobInfo struct {
        RunTimeTicks     *int64
        BaseRequest      struct {
                StartTimeTicks *int64
        }
        ReportTranscodingProgress func(transcodingPosition *time.Duration, framerate *float32, percent *float64, bytesTranscoded *int64, bitRate *int)
        MediaPath        string
}
*/

type JobLogger struct {
	logger *log.Logger
}

func NewJobLogger() *JobLogger {
	return &JobLogger{
		logger: log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds),
	}
}

// func (j *JobLogger) StartStreamingLog(state mediaencoding.EncodingJobInfo, input io.Reader, output io.Writer) {
func (j *JobLogger) StartStreamingLog(state streaming.StreamState, input io.Reader, output io.Writer) {
	// Create a buffer to read from the input stream
	//      buf := make([]byte, 1024)
	reader := bufio.NewReader(input)

	// Read from the input stream and write to the output stream
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// We've reached the end of the file
				klog.Infoln("End of stream reached")
				return
			} else {
				j.logger.Printf("Error reading from input stream: %v", err)
				return
			}
		}

		ParseLogLine(line, state)

		klog.Infof("--> %s", line)
		_, err = output.Write([]byte(line))
		if err != nil {
			j.logger.Printf("Error writing to output stream: %v", err)
			return
		}
	}
}

// func ParseLogLine(line string, state mediaencoding.EncodingJobInfo) {
func ParseLogLine(line string, state streaming.StreamState) {
	var framerate *float32
	var percent *float64
	var transcodingPosition *time.Duration
	var bytesTranscoded *int64
	var bitRate *int

	parts := strings.Split(line, " ")

	var totalMs float64
	if state.RunTimeTicks != nil {
		totalMs = float64(time.Duration(*state.RunTimeTicks * 100).Milliseconds())
	}

	var startMs float64
	if state.BaseRequest.StartTimeTicks != nil {
		startMs = float64(time.Duration(*state.BaseRequest.StartTimeTicks * 100).Milliseconds())
	}

	for i, part := range parts {
		if strings.EqualFold(part, "fps=") && i+1 < len(parts) {
			rate, err := strconv.ParseFloat(parts[i+1], 32)
			if err == nil {
				framerate = new(float32)
				*framerate = float32(rate)
			}
		} else if strings.HasPrefix(part, "fps=") {
			rate, err := strconv.ParseFloat(strings.SplitN(part, "=", 2)[1], 32)
			if err == nil {
				framerate = new(float32)
				*framerate = float32(rate)
			}
		} else if state.RunTimeTicks != nil && strings.HasPrefix(part, "time=") {
			timeStr := strings.SplitN(part, "=", 2)[1]
			// timeVal, err := time.ParseDuration(timeStr)
			timeVal, err := parseFfmpegTime(timeStr)
			klog.Infoln(timeVal)
			if err == nil {
				currentMs := startMs + float64(timeVal.Milliseconds())
				percent = new(float64)
				*percent = 100.0 * currentMs / totalMs
				transcodingPosition = new(time.Duration)
				*transcodingPosition = time.Duration(currentMs * float64(time.Millisecond))
			} else {
				klog.Infoln(err)
			}
		} else if strings.HasPrefix(part, "size=") {
			size := strings.SplitN(part, "=", 2)[1]
			var scale int64 = 1
			if strings.Contains(size, "kb") {
				scale = 1024
				size = strings.Replace(size, "kb", "", 1)
			}
			sizeVal, err := strconv.ParseInt(size, 10, 64)
			if err == nil {
				bytesTranscoded = new(int64)
				*bytesTranscoded = sizeVal * scale
			}
		} else if strings.HasPrefix(part, "bitrate=") {
			rate := strings.SplitN(part, "=", 2)[1]
			var scale int64 = 1
			if strings.Contains(rate, "kbits/s") {
				scale = 1024
				rate = strings.Replace(rate, "kbits/s", "", 1)
			}
			rateVal, err := strconv.ParseFloat(rate, 32)
			if err == nil {
				bitRate = new(int)
				*bitRate = int(math.Ceil(rateVal * float64(scale)))
			}
		}
	}

	if framerate != nil || percent != nil {
		state.ReportTranscodingProgress(transcodingPosition, framerate, percent, bytesTranscoded, bitRate)
	}
}

func parseFfmpegTime(timeStr string) (time.Duration, error) {

	// Parse the time component
	timeParts := strings.Split(timeStr, ":")
	if len(timeParts) != 3 {
		return 0, fmt.Errorf("invalid FFmpeg time format: %s", timeStr)
	}

	hours, _ := time.ParseDuration(timeParts[0] + "h")
	minutes, _ := time.ParseDuration(timeParts[1] + "m")
	seconds, err := time.ParseDuration(timeParts[2] + "s")
	if err != nil {
		return 0, err
	}

	return hours + minutes + seconds, nil
}

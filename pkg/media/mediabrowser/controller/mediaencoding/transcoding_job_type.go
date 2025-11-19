package mediaencoding

type TranscodingJobType int

const (
	// Progressive represents a progressive transcoding job.
	Progressive TranscodingJobType = iota
	// Hls represents an HLS (HTTP Live Streaming) transcoding job.
	Hls
	// Dash represents a DASH (Dynamic Adaptive Streaming over HTTP) transcoding job.
	Dash
)

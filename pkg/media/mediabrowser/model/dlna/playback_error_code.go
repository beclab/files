package dlna

type PlaybackErrorCode int

const (
	NotAllowed PlaybackErrorCode = iota
	NoCompatibleStream
	RateLimitExceeded
)

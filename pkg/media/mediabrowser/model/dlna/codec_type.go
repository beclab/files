package dlna

type CodecType int

const (
	CodecType_Video CodecType = iota
	CodecType_VideoAudio
	CodecType_Audio
)

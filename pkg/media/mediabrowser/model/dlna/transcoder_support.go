package dlna

type ITranscoderSupport interface {
	CanEncodeToAudioCodec(codec string) bool
	CanEncodeToSubtitleCodec(codec string) bool
	CanExtractSubtitles(codec string) bool
}

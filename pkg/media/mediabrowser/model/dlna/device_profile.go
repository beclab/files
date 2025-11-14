package dlna

type DeviceProfile struct {
	Name                             *string
	Id                               *string
	MaxStreamingBitrate              *int
	MaxStaticBitrate                 *int
	MusicStreamingTranscodingBitrate *int
	MaxStaticMusicBitrate            *int
	DirectPlayProfiles               []DirectPlayProfile
	TranscodingProfiles              []TranscodingProfile
	ContainerProfiles                []ContainerProfile
	CodecProfiles                    []CodecProfile
	SubtitleProfiles                 []SubtitleProfile
}

func NewDeviceProfile() *DeviceProfile {
	return &DeviceProfile{
		MaxStreamingBitrate:              intPtr(8000000),
		MaxStaticBitrate:                 intPtr(8000000),
		MusicStreamingTranscodingBitrate: intPtr(128000),
		MaxStaticMusicBitrate:            intPtr(8000000),
	}
}

func intPtr(v int) *int {
	return &v
}

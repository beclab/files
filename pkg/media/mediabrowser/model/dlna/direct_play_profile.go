package dlna

type DirectPlayProfile struct {
	Container  *string
	AudioCodec *string
	VideoCodec *string
	Type       DlnaProfileType
}

func (dp *DirectPlayProfile) SupportsContainer(container *string) bool {
	return ContainsContainer(dp.Container, container)
}

/*
func (dp *DirectPlayProfile) SupportsVideoCodec(codec *string) bool {
    return dp.Type == Video && ContainerProfile.ContainsContainer(dp.VideoCodec, codec)
}

func (dp *DirectPlayProfile) SupportsAudioCodec(codec *string) bool {
    return (dp.Type == Audio || dp.Type == Video) && ContainerProfile.ContainsContainer(dp.AudioCodec, codec)
}
*/

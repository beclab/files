package streaming

type VideoRequestDto struct {
	// Embedded struct to inherit properties from StreamingRequestDto
	*StreamingRequestDto

	HasFixedResolution bool

	EnableSubtitlesInManifest bool

	EnableTrickplay bool
}

func NewVideoRequestDto(width, height *int) *VideoRequestDto {
	return &VideoRequestDto{
		HasFixedResolution: width != nil || height != nil,
	}
}

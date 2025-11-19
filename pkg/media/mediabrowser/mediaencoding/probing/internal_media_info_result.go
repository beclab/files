package probing

type InternalMediaInfoResult struct {
	Streams  []*MediaStreamInfo `json:"streams"`
	Format   *MediaFormatInfo   `json:"format"`
	Chapters []*MediaChapter    `json:"chapters"`
}

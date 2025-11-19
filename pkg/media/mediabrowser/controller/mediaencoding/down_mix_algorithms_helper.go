package mediaencoding

import (
	"files/pkg/media/mediabrowser/model/entities"
)

// DownMixAlgorithmsHelper provides methods for downmix algorithm capabilities.
type DownMixAlgorithmsHelper struct{}

type AlgorithmFilterKey struct {
	Algorithm     entities.DownMixStereoAlgorithms
	ChannelLayout string
}

// AlgorithmFilterStrings maps (algorithm, layout) to FFmpeg filter strings.
var AlgorithmFilterStrings = map[AlgorithmFilterKey]string{
	{entities.Dave750, "5.1"}:           "pan=stereo|c0=0.5*c2+0.707*c0+0.707*c4+0.5*c3|c1=0.5*c2+0.707*c1+0.707*c5+0.5*c3",
	{entities.Dave750, "7.1"}:           "pan=5.1(side)|c0=c0|c1=c1|c2=c2|c3=c3|c4=0.707*c4+0.707*c6|c5=0.707*c5+0.707*c7,pan=stereo|c0=0.5*c2+0.707*c0+0.707*c4+0.5*c3|c1=0.5*c2+0.707*c1+0.707*c5+0.5*c3",
	{entities.NightmodeDialogue, "5.1"}: "pan=stereo|c0=c2+0.30*c0+0.30*c4|c1=c2+0.30*c1+0.30*c5",
	{entities.NightmodeDialogue, "7.1"}: "pan=5.1(side)|c0=c0|c1=c1|c2=c2|c3=c3|c4=0.707*c4+0.707*c6|c5=0.707*c5+0.707*c7,pan=stereo|c0=c2+0.30*c0+0.30*c4|c1=c2+0.30*c1+0.30*c5",
	{entities.Rfc7845, "3.0"}:           "pan=stereo|c0=0.414214*c2+0.585786*c0|c1=0.414214*c2+0.585786*c1",
	{entities.Rfc7845, "quad"}:          "pan=stereo|c0=0.422650*c0+0.366025*c2+0.211325*c3|c1=0.422650*c1+0.366025*c3+0.211325*c2",
	{entities.Rfc7845, "5.0"}:           "pan=stereo|c0=0.460186*c2+0.650802*c0+0.563611*c3+0.325401*c4|c1=0.460186*c2+0.650802*c1+0.563611*c4+0.325401*c3",
	{entities.Rfc7845, "5.1"}:           "pan=stereo|c0=0.374107*c2+0.529067*c0+0.458186*c4+0.264534*c5+0.374107*c3|c1=0.374107*c2+0.529067*c1+0.458186*c5+0.264534*c4+0.374107*c3",
	{entities.Rfc7845, "6.1"}:           "pan=stereo|c0=0.321953*c2+0.455310*c0+0.394310*c5+0.227655*c6+0.278819*c4+0.321953*c3|c1=0.321953*c2+0.455310*c1+0.394310*c6+0.227655*c5+0.278819*c4+0.321953*c3",
	{entities.Rfc7845, "7.1"}:           "pan=stereo|c0=0.274804*c2+0.388631*c0+0.336565*c6+0.194316*c7+0.336565*c4+0.194316*c5+0.274804*c3|c1=0.274804*c2+0.388631*c1+0.336565*c7+0.194316*c6+0.336565*c5+0.194316*c4+0.274804*c3",
	{entities.Ac4, "3.0"}:               "pan=stereo|c0=c0+0.707*c2|c1=c1+0.707*c2",
	{entities.Ac4, "5.0"}:               "pan=stereo|c0=c0+0.707*c2+0.707*c3|c1=c1+0.707*c2+0.707*c4",
	{entities.Ac4, "5.1"}:               "pan=stereo|c0=c0+0.707*c2+0.707*c4|c1=c1+0.707*c2+0.707*c5",
	{entities.Ac4, "7.0"}:               "pan=5.0(side)|c0=c0|c1=c1|c2=c2|c3=0.707*c3+0.707*c5|c4=0.707*c4+0.707*c6,pan=stereo|c0=c0+0.707*c2+0.707*c3|c1=c1+0.707*c2+0.707*c4",
	{entities.Ac4, "7.1"}:               "pan=5.1(side)|c0=c0|c1=c1|c2=c2|c3=c3|c4=0.707*c4+0.707*c6|c5=0.707*c5+0.707*c7,pan=stereo|c0=c0+0.707*c2+0.707*c4|c1=c1+0.707*c2+0.707*c5",
}

// InferChannelLayout returns the audio channel layout string from the audio stream.
// If the input audio stream does not have a valid layout string, it guesses from the channel count.
func (h DownMixAlgorithmsHelper) InferChannelLayout(audioStream entities.MediaStream) string {
	if audioStream.ChannelLayout != "" {
		// Note: BDMVs do not derive this string from ffmpeg, which would cause ambiguity with 4-channel audio
		// "quad" => 2 front and 2 rear, "4.0" => 3 front and 1 rear
		// BDMV will always use "4.0" in this case
		// Because the quad layout is super rare in BDs, we will use "4.0" as is here
		return audioStream.ChannelLayout
	}

	if audioStream.Channels == nil {
		return ""
	}

	// When we don't have definitive channel layout, we have to guess from the channel count
	// Guessing is not always correct, but for most videos we don't have to guess like this as the definitive layout is recorded during scan
	switch *audioStream.Channels {
	case 1:
		return "mono"
	case 2:
		return "stereo"
	case 3:
		return "2.1" // Could also be 3.0, prefer 2.1
	case 4:
		return "4.0" // Could also be quad (with rear left and rear right) or 3.1 with LFE, prefer 4.0 with front center and back center
	case 5:
		return "5.0"
	case 6:
		return "5.1" // Could also be 6.0 or hexagonal, prefer 5.1
	case 7:
		return "6.1" // Could also be 7.0, prefer 6.1
	case 8:
		return "7.1" // Could also be 8.0, prefer 7.1
	default:
		return "" // Return empty string for unsupported layout
	}
}

package dlna

import (
	"math"
)

type ResolutionConfiguration struct {
	MaxWidth   int
	MaxBitrate int
}

/*
var Configurations = []ResolutionConfiguration{
	{426, 320000},
	{640, 400000},
	{720, 950000},
	{1280, 2500000},
	{1920, 4000000},
	{2560, 20000000},
	{3840, 35000000},
}
*/

var Configurations = []ResolutionConfiguration{
	{416, 365000},
	{640, 730000},
	{768, 1100000},
	{960, 3000000},
	{1280, 6000000},
	{1920, 13500000},
	{2560, 28000000},
	{3840, 50000000},
}

type ResolutionOptions struct {
	MaxWidth  *int
	MaxHeight *int
}

func Normalize(inputBitrate *int, outputBitrate, h264EquivalentOutputBitrate int, maxWidth, maxHeight *int, targetFps *float32, isHdr bool) ResolutionOptions {
	// If the bitrate isn't changing, then don't downscale the resolution
	if inputBitrate != nil && outputBitrate >= *inputBitrate {
		if maxWidth != nil || maxHeight != nil {
			return ResolutionOptions{
				MaxWidth:  maxWidth,
				MaxHeight: maxHeight,
			}
		}
	}

	// Our reference bitrate is based on SDR h264 at 30fps
	var referenceFps float32 = 30.0
	if targetFps != nil {
		referenceFps = *targetFps
	}

	var referenceScale float32
	if referenceFps <= 30.0 {
		referenceScale = 30.0 / referenceFps
	} else {
		referenceScale = 1.0 / float32(math.Sqrt(float64(referenceFps/30.0)))
	}

	referenceBitrate := float32(h264EquivalentOutputBitrate) * referenceScale

	if isHdr {
		referenceBitrate *= 0.8
	}
	resolutionConfig := getResolutionConfiguration(int(referenceBitrate))
	if resolutionConfig == nil {
		return ResolutionOptions{
			MaxWidth:  maxWidth,
			MaxHeight: maxHeight,
		}
	}
	originWidthValue := maxWidth
	var finalMaxWidth int
	if maxWidth != nil {
		finalMaxWidth = int(math.Min(float64(resolutionConfig.MaxWidth), float64(*maxWidth)))
	} else {
		finalMaxWidth = resolutionConfig.MaxWidth
	}
	var finalMaxHeight *int
	if originWidthValue == nil || (maxWidth != nil && *originWidthValue != finalMaxWidth) {
		finalMaxHeight = nil
	} else {
		finalMaxHeight = maxHeight
	}

	return ResolutionOptions{
		MaxWidth:  &finalMaxWidth,
		MaxHeight: finalMaxHeight,
	}
}

/*
func getResolutionConfiguration(outputBitrate int) *ResolutionConfiguration {
	var previousOption *ResolutionConfiguration
	for _, config := range Configurations {
		if outputBitrate <= config.MaxBitrate {
			if previousOption != nil {
				return previousOption
			}
			return &config
		}
		previousOption = &config
	}
	return nil
}
*/

func getResolutionConfiguration(outputBitrate int) *ResolutionConfiguration {
	for _, config := range Configurations {
		if outputBitrate <= config.MaxBitrate {
			return &config
		}
	}
	return nil
}

func minInt(a, b, c int) int {
	if a < b {
		return a
	}
	if b < c {
		return b
	}
	return c
}

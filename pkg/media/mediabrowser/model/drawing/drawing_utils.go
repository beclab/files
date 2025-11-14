package drawing

import (
	"math"
)

func Resize(size ImageDimensions, width, height, maxWidth, maxHeight int) ImageDimensions {
	newWidth, newHeight := size.Width, size.Height

	if width > 0 && height > 0 {
		newWidth, newHeight = width, height
	} else if height > 0 {
		newWidth = getNewWidth(newHeight, newWidth, height)
		newHeight = height
	} else if width > 0 {
		newHeight = getNewHeight(newHeight, newWidth, width)
		newWidth = width
	}

	if maxHeight > 0 && maxHeight < newHeight {
		newWidth = getNewWidth(newHeight, newWidth, maxHeight)
		newHeight = maxHeight
	}

	if maxWidth > 0 && maxWidth < newWidth {
		newHeight = getNewHeight(newHeight, newWidth, maxWidth)
		newWidth = maxWidth
	}

	return ImageDimensions{Width: newWidth, Height: newHeight}
}

func ResizeFill(size ImageDimensions, fillWidth, fillHeight *int) ImageDimensions {
	// Return original size if input is invalid.
	if (fillWidth == nil || *fillWidth == 0) && (fillHeight == nil || *fillHeight == 0) {
		return size
	}

	if fillWidth == nil || *fillWidth == 0 {
		*fillWidth = 1
	}

	if fillHeight == nil || *fillHeight == 0 {
		*fillHeight = 1
	}

	widthRatio := float64(size.Width) / float64(*fillWidth)
	heightRatio := float64(size.Height) / float64(*fillHeight)
	scaleRatio := math.Min(widthRatio, heightRatio)

	// Clamp to current size.
	if scaleRatio < 1 {
		return size
	}

	newWidth := int(math.Ceil(float64(size.Width) / scaleRatio))
	newHeight := int(math.Ceil(float64(size.Height) / scaleRatio))

	return ImageDimensions{Width: newWidth, Height: newHeight}
}

func getNewWidth(currentHeight, currentWidth, newHeight int) int {
	return int(float64(newHeight) / float64(currentHeight) * float64(currentWidth))
}

func getNewHeight(currentHeight, currentWidth, newWidth int) int {
	return int(float64(newWidth) / float64(currentWidth) * float64(currentHeight))
}

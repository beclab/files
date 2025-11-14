package drawing

import (
	"fmt"
)

type ImageDimensions struct {
	Width  int
	Height int
}

func NewImageDimensions(width, height int) ImageDimensions {
	return ImageDimensions{
		Width:  width,
		Height: height,
	}
}

func (d ImageDimensions) Equals(other ImageDimensions) bool {
	return d.Width == other.Width && d.Height == other.Height
}

func (d ImageDimensions) String() string {
	return fmt.Sprintf("%d-%d", d.Width, d.Height)
}

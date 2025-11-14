package library

import (
	"io"
)

type IDirectStreamProvider interface {
	GetStream() io.ReadSeeker
}

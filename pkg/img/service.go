//go:generate go-enum --sql --marshal --file $GOFILE
package img

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	_ "image/png"
	"io"

	"github.com/disintegration/imaging"
	"github.com/dsoprea/go-exif/v3"
	"github.com/marusama/semaphore/v2"
	"github.com/nfnt/resize"
	"k8s.io/klog/v2"

	exifcommon "github.com/dsoprea/go-exif/v3/common"
)

var imgSvc *Service

// ErrUnsupportedFormat means the given image format is not supported.
var ErrUnsupportedFormat = errors.New("unsupported image format")

// Service
type Service struct {
	sem semaphore.Semaphore
}

func New(workers int) *Service {
	imgSvc = &Service{
		sem: semaphore.New(workers),
	}
	return imgSvc
}

func GetImageService() *Service {
	return imgSvc
}

// Format is an image file format.
/*
ENUM(
jpeg
png
gif
tiff
bmp
)
*/
type Format int

func (x Format) toImaging() imaging.Format {
	switch x {
	case FormatJpeg:
		return imaging.JPEG
	case FormatPng:
		return imaging.PNG
	case FormatGif:
		return imaging.GIF
	case FormatTiff:
		return imaging.TIFF
	case FormatBmp:
		return imaging.BMP
	default:
		return imaging.JPEG
	}
}

/*
ENUM(
high
medium
low
)
*/
type Quality int

func (x Quality) resampleFilter() imaging.ResampleFilter {
	switch x {
	case QualityHigh:
		return imaging.Lanczos
	case QualityMedium:
		return imaging.Box
	case QualityLow:
		return imaging.NearestNeighbor
	default:
		return imaging.Box
	}
}

/*
ENUM(
fit
fill
)
*/
type ResizeMode int

func (s *Service) FormatFromExtension(ext string) (Format, error) {
	format, err := imaging.FormatFromExtension(ext)
	if err != nil {
		return -1, ErrUnsupportedFormat
	}
	switch format {
	case imaging.JPEG:
		return FormatJpeg, nil
	case imaging.PNG:
		return FormatPng, nil
	case imaging.GIF:
		return FormatGif, nil
	case imaging.TIFF:
		return FormatTiff, nil
	case imaging.BMP:
		return FormatBmp, nil
	}
	return -1, ErrUnsupportedFormat
}

type resizeConfig struct {
	format     Format
	resizeMode ResizeMode
	quality    Quality
}

type Option func(*resizeConfig)

func WithFormat(format Format) Option {
	return func(config *resizeConfig) {
		config.format = format
	}
}

func WithMode(mode ResizeMode) Option {
	return func(config *resizeConfig) {
		config.resizeMode = mode
	}
}

func WithQuality(quality Quality) Option {
	return func(config *resizeConfig) {
		config.quality = quality
	}
}

func (s *Service) Resize2(ctx context.Context, in io.Reader, width, height int, out io.Writer, options ...Option) error {
	if err := s.sem.Acquire(ctx, 1); err != nil {
		return err
	}
	defer s.sem.Release(1)

	format, wrappedReader, err := s.detectFormat(in)
	if err != nil {
		klog.Errorln("Detect format:", err)
		return err
	}

	config := resizeConfig{
		format:     format,
		resizeMode: ResizeModeFit,
		quality:    QualityMedium,
	}
	klog.Infoln("format: ", format)
	for _, option := range options {
		option(&config)
	}

	if config.quality == QualityLow && format == FormatJpeg {
		thm, newWrappedReader, errThm := getEmbeddedThumbnail(wrappedReader)
		wrappedReader = newWrappedReader
		if errThm == nil {
			klog.Errorln("Get Embedded Thumbnail: ", err)
			_, err = out.Write(thm)
			if err == nil {
				return nil
			}
		}
	}

	img, _, err := image.Decode(wrappedReader)
	if err != nil {
		klog.Errorln("Decode:", err)
		return err
	}

	resizedImg := resize.Resize(uint(width), uint(height), img, resize.Lanczos3)

	return png.Encode(out, resizedImg)
}

func (s *Service) Resize(ctx context.Context, in io.Reader, width, height int, out io.Writer, options ...Option) error {
	if err := s.sem.Acquire(ctx, 1); err != nil {
		return err
	}
	defer s.sem.Release(1)

	format, wrappedReader, err := s.detectFormat(in)
	if err != nil {
		klog.Errorln("Detect format:", err)
		return err
	}

	config := resizeConfig{
		format:     format,
		resizeMode: ResizeModeFit,
		quality:    QualityMedium,
	}
	klog.Infoln("format:", format)
	for _, option := range options {
		option(&config)
	}

	if config.quality == QualityLow && format == FormatJpeg {
		thm, newWrappedReader, errThm := getEmbeddedThumbnail(wrappedReader)
		wrappedReader = newWrappedReader
		if errThm == nil {
			klog.Errorln("Get Embedded Thumbnail: ", err)
			_, err = out.Write(thm)
			if err == nil {
				return nil
			}
		}
	}

	img, err := imaging.Decode(wrappedReader, imaging.AutoOrientation(true))
	if err != nil {
		klog.Errorln("Decode:", err)
		return err
	}

	switch config.resizeMode {
	case ResizeModeFill:
		img = imaging.Fill(img, width, height, imaging.Center, config.quality.resampleFilter())
	case ResizeModeFit:
		fallthrough
	default:
		img = imaging.Fit(img, width, height, config.quality.resampleFilter())
	}

	return imaging.Encode(out, img, config.format.toImaging())
}

func (s *Service) detectFormat(in io.Reader) (Format, io.Reader, error) {
	buf := &bytes.Buffer{}
	r := io.TeeReader(in, buf)

	_, imgFormat, err := image.DecodeConfig(r)
	if err != nil {
		return 0, nil, fmt.Errorf("%s: %w", err.Error(), ErrUnsupportedFormat)
	}

	format, err := ParseFormat(imgFormat)
	if err != nil {
		return 0, nil, ErrUnsupportedFormat
	}

	return format, io.MultiReader(buf, in), nil
}

func getEmbeddedThumbnail(in io.Reader) ([]byte, io.Reader, error) {
	buf := &bytes.Buffer{}
	r := io.TeeReader(in, buf)
	wrappedReader := io.MultiReader(buf, in)

	offset := 0
	offsets := []int{12, 30}
	head := make([]byte, 0xffff)

	_, err := r.Read(head)
	if err != nil {
		return nil, wrappedReader, err
	}

	for _, offset = range offsets {
		if _, err = exif.ParseExifHeader(head[offset:]); err == nil {
			break
		}
	}

	if err != nil {
		return nil, wrappedReader, err
	}

	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		return nil, wrappedReader, err
	}

	_, index, err := exif.Collect(im, exif.NewTagIndex(), head[offset:])
	if err != nil {
		return nil, wrappedReader, err
	}

	ifd := index.RootIfd.NextIfd()
	if ifd == nil {
		return nil, wrappedReader, exif.ErrNoThumbnail
	}

	thm, err := ifd.Thumbnail()
	return thm, wrappedReader, err
}

package entities

import (
	"strconv"
	"strings"

	"golang.org/x/text/language"

	//	ffmpeg "github.com/u2takey/ffmpeg-go"
	"files/pkg/media/jellyfin/data/enums"
	"files/pkg/media/mediabrowser/model/dlna"
	funk "github.com/thoas/go-funk"
)

var SpecialCodes = []string{
	"mis", // Uncoded languages
	"mul", // Multiple languages
	"und", // Undetermined
	"zxx", // No linguistic content; not applicable
}

type MediaStream struct {
	Codec                     string
	CodecTag                  string
	Language                  string
	ColorRange                string
	ColorSpace                string
	ColorTransfer             string
	ColorPrimaries            string
	DvVersionMajor            *int
	DvVersionMinor            *int
	DvProfile                 *int
	DvLevel                   *int
	RpuPresentFlag            *int
	ElPresentFlag             *int
	BlPresentFlag             *int
	DvBlSignalCompatibilityId *int
	Comment                   string
	TimeBase                  string
	CodecTimeBase             string
	Title                     string
	VideoRange                enums.VideoRange
	VideoRangeType            enums.VideoRangeType
	VideoDoViTitle            string
	AudioSpatialFormat        AudioSpatialFormat
	LocalizedUndefined        string
	LocalizedDefault          string
	LocalizedForced           string
	LocalizedExternal         string
	LocalizedHearingImpaired  string
	DisplayTitle              string
	NalLengthSize             string
	IsInterlaced              bool
	IsAVC                     *bool
	ChannelLayout             string
	BitRate                   *int
	BitDepth                  *int
	RefFrames                 *int
	PacketLength              *int
	Channels                  *int
	SampleRate                *int
	IsDefault                 bool
	IsForced                  bool
	IsHearingImpaired         bool
	Height                    *int
	Width                     *int
	AverageFrameRate          *float32
	RealFrameRate             *float32
	ReferenceFrameRate        *float32
	Profile                   string
	Type                      MediaStreamType
	AspectRatio               string
	Index                     int
	Score                     *int
	IsExternal                bool
	DeliveryMethod            *dlna.SubtitleDeliveryMethod
	DeliveryUrl               string
	IsExternalUrl             *bool
	SupportsExternalStream    bool
	Path                      string
	PixelFormat               string
	Level                     *float64
	IsAnamorphic              *bool
	Rotation                  *int
}

func (m *MediaStream) GetVideoRange() enums.VideoRange {
	videoRange, _ := m.GetVideoColorRange()
	return videoRange
}

func (m *MediaStream) GetVideoRangeType() enums.VideoRangeType {
	_, videoRangeType := m.GetVideoColorRange()
	return videoRangeType
}

type AudioSpatialFormat int

const (
	AudioSpatialFormatNone AudioSpatialFormat = iota
	AudioSpatialFormatDolbyAtmos
	AudioSpatialFormatDTSX
)

func (m *MediaStream) GetVideoDoViTitle() string {
	var dvProfile int
	var rpuPresentFlag, blPresentFlag bool
	var dvBlCompatId int

	if rpuPresentFlag && blPresentFlag && (dvProfile == 4 || dvProfile == 5 || dvProfile == 7 || dvProfile == 8 || dvProfile == 9) {
		title := "DV Profile " + strconv.Itoa(dvProfile)
		if dvBlCompatId > 0 {
			title += "." + strconv.Itoa(dvBlCompatId)
		}
		switch dvBlCompatId {
		case 1:
			return title + " (HDR10)"
		case 2:
			return title + " (SDR)"
		case 4:
			return title + " (HLG)"
		default:
			return title
		}
	}
	return ""
}

func (m *MediaStream) GetAudioSpatialFormat() AudioSpatialFormat {
	if m.Type != MediaStreamTypeAudio || m.Profile == "" {
		return AudioSpatialFormatNone
	}

	if strings.Contains(strings.ToLower(m.Profile), strings.ToLower("Dolby Atmos")) {
		return AudioSpatialFormatDolbyAtmos
	} else if strings.Contains(strings.ToLower(m.Profile), strings.ToLower("DTS:X")) {
		return AudioSpatialFormatDTSX
	} else {
		return AudioSpatialFormatNone
	}
}

func getFullLanguageName(languageCode string) (string, error) {
	tag, err := language.Parse(languageCode)
	if err != nil {
		return "", err
	}

	name := tag.String()
	return name, nil
}

func getAudioCodecFriendlyName(codecName string) (string, error) {
	/*
	   codecDetails, err := ffmpeg.GetCodecDetails(codecName)
	   if err != nil {
	       return "", err
	   }

	   return codecDetails.LongName, nil
	*/

	return "todo", nil
}

func (m *MediaStream) GetDisplayTitle() string {
	switch m.Type {
	case MediaStreamTypeAudio:
		attributes := []string{}
		if m.Language != "" && !funk.Contains(SpecialCodes, strings.ToLower(m.Language)) {
			fullLanguage, _ := getFullLanguageName(m.Language)
			attributes = append(attributes, strings.ToUpper(fullLanguage))
		}
		if m.Profile != "" && m.Profile != "lc" {
			attributes = append(attributes, m.Profile)
		} else if m.Codec != "" {
			//            attributes = append(attributes, getAudioCodecFriendlyName(m.Codec))
			name, _ := getAudioCodecFriendlyName(m.Codec)
			attributes = append(attributes, name)
		}
		if m.ChannelLayout != "" {
			attributes = append(attributes, strings.ToUpper(m.ChannelLayout))
		} else if m.Channels != nil {
			attributes = append(attributes, strconv.Itoa(*m.Channels)+" ch")
		}
		if m.IsDefault {
			if m.LocalizedDefault != "" {
				attributes = append(attributes, m.LocalizedDefault)
			} else {
				attributes = append(attributes, "Default")
			}
		}
		if m.IsExternal {
			if m.LocalizedExternal != "" {
				attributes = append(attributes, m.LocalizedExternal)
			} else {
				attributes = append(attributes, "External")
			}
		}
		if m.Title != "" {
			result := strings.Builder{}
			result.WriteString(m.Title)
			for _, tag := range attributes {
				if !strings.Contains(strings.ToLower(m.Title), strings.ToLower(tag)) {
					result.WriteString(" - ")
					result.WriteString(tag)
				}
			}
			return result.String()
		}
		return strings.Join(attributes, " - ")
	case MediaStreamTypeVideo:
		attributes := []string{}
		resolutionText := m.getResolutionText()
		if resolutionText != "" {
			attributes = append(attributes, resolutionText)
		}
		if m.Codec != "" {
			attributes = append(attributes, strings.ToUpper(m.Codec))
		}
		/* comiple
		   if m.VideoRange != enums.Unknown {
		       attributes = append(attributes, m.VideoRange.String())
		   }
		*/
		if m.Title != "" {
			result := strings.Builder{}
			result.WriteString(m.Title)
			for _, tag := range attributes {
				if !strings.Contains(strings.ToLower(m.Title), strings.ToLower(tag)) {
					result.WriteString(" - ")
					result.WriteString(tag)
				}
			}
			return result.String()
		}
		return strings.Join(attributes, " ")
	case MediaStreamTypeSubtitle:
		attributes := []string{}
		if m.Language != "" {
			fullLanguage, _ := getFullLanguageName(m.Language)
			attributes = append(attributes, strings.ToUpper(fullLanguage))
		} else {
			if m.LocalizedUndefined != "" {
				attributes = append(attributes, m.LocalizedUndefined)
			} else {
				attributes = append(attributes, "Und")
			}
		}
		if m.IsHearingImpaired {
			if m.LocalizedHearingImpaired != "" {
				attributes = append(attributes, m.LocalizedHearingImpaired)
			} else {
				attributes = append(attributes, "Hearing Impaired")
			}
		}
		if m.IsDefault {
			if m.LocalizedDefault != "" {
				attributes = append(attributes, m.LocalizedDefault)
			} else {
				attributes = append(attributes, "Default")
			}
		}
		if m.IsForced {
			if m.LocalizedForced != "" {
				attributes = append(attributes, m.LocalizedForced)
			} else {
				attributes = append(attributes, "Forced")
			}
		}
		if m.Codec != "" {
			attributes = append(attributes, strings.ToUpper(m.Codec))
		}
		if m.IsExternal {
			if m.LocalizedExternal != "" {
				attributes = append(attributes, m.LocalizedExternal)
			} else {
				attributes = append(attributes, "External")
			}
		}
		if m.Title != "" {
			result := strings.Builder{}
			result.WriteString(m.Title)
			for _, tag := range attributes {
				if !strings.Contains(strings.ToLower(m.Title), strings.ToLower(tag)) {
					result.WriteString(" - ")
					result.WriteString(tag)
				}
			}
			return result.String()
		}
		return strings.Join(attributes, " - ")
	default:
		return ""
	}
}

func (m *MediaStream) IsTextSubtitleStream() bool {
	if m.Type != MediaStreamTypeSubtitle {
		return false
	}

	if m.Codec == "" && !m.IsExternal {
		return false
	}

	return isTextFormat(m.Codec)
}

func (m *MediaStream) getResolutionText() string {
	if m.Width == nil || m.Height == nil {
		return ""
	}

	switch {
	case *m.Width <= 256 && *m.Height <= 144:
		if m.IsInterlaced {
			return "144i"
		}
		return "144p"
	case *m.Width <= 426 && *m.Height <= 240:
		if m.IsInterlaced {
			return "240i"
		}
		return "240p"
	case *m.Width <= 640 && *m.Height <= 360:
		if m.IsInterlaced {
			return "360i"
		}
		return "360p"
	case *m.Width <= 682 && *m.Height <= 384:
		if m.IsInterlaced {
			return "384i"
		}
		return "384p"
	case *m.Width <= 720 && *m.Height <= 404:
		if m.IsInterlaced {
			return "404i"
		}
		return "404p"
	case *m.Width <= 854 && *m.Height <= 480:
		if m.IsInterlaced {
			return "480i"
		}
		return "480p"
	case *m.Width <= 960 && *m.Height <= 544:
		if m.IsInterlaced {
			return "540i"
		}
		return "540p"
	case *m.Width <= 1024 && *m.Height <= 576:
		if m.IsInterlaced {
			return "576i"
		}
		return "576p"
	case *m.Width <= 1280 && *m.Height <= 962:
		if m.IsInterlaced {
			return "720i"
		}
		return "720p"
	case *m.Width <= 2560 && *m.Height <= 1440:
		if m.IsInterlaced {
			return "1080i"
		}
		return "1080p"
	case *m.Width <= 4096 && *m.Height <= 3072:
		return "4K"
	case *m.Width <= 8192 && *m.Height <= 6144:
		return "8K"
	default:
		return ""
	}
}

func isTextFormat(codec string) bool {
	// Implement the logic to check if the given codec is a text-based format
	// For example, you can check if the codec is "subrip", "ass", "ssa", etc.
	// Return true if it is a text-based format, false otherwise.
	return false
}

func IsTextFormat(format string) bool {
	codec := format
	if codec == "" {
		codec = "empty"
	}

	return !strings.Contains(strings.ToLower(codec), "pgs") &&
		!strings.Contains(strings.ToLower(codec), "dvd") &&
		!strings.Contains(strings.ToLower(codec), "dvbsub") &&
		!strings.EqualFold(codec, "sub") &&
		!strings.EqualFold(codec, "sup") &&
		!strings.EqualFold(codec, "dvb_subtitle")
}

func SupportsSubtitleConversionTo(isTextSubtitleStream bool, fromCodec, toCodec string) bool {
	if !isTextSubtitleStream {
		return false
	}

	// Can't convert from this
	if strings.EqualFold(fromCodec, "ass") {
		return false
	}

	if strings.EqualFold(fromCodec, "ssa") {
		return false
	}

	// Can't convert to this
	if strings.EqualFold(toCodec, "ass") {
		return false
	}

	if strings.EqualFold(toCodec, "ssa") {
		return false
	}

	return true
}

func (m *MediaStream) GetVideoColorRange() (enums.VideoRange, enums.VideoRangeType) {
	if m.Type != MediaStreamTypeVideo {
		return enums.VideoRangeUnknown, enums.VideoRangeTypeUnknown
	}

	isDoViProfile := *m.DvProfile == 5 || *m.DvProfile == 7 || *m.DvProfile == 8
	isDoViFlag := *m.RpuPresentFlag == 1 && *m.BlPresentFlag == 1 && (*m.DvBlSignalCompatibilityId == 0 || *m.DvBlSignalCompatibilityId == 1 || *m.DvBlSignalCompatibilityId == 4 || *m.DvBlSignalCompatibilityId == 2 || *m.DvBlSignalCompatibilityId == 6)

	switch {
	case (isDoViProfile && isDoViFlag) ||
		strings.EqualFold(m.CodecTag, "dovi") ||
		strings.EqualFold(m.CodecTag, "dvh1") ||
		strings.EqualFold(m.CodecTag, "dvhe") ||
		strings.EqualFold(m.CodecTag, "dav1"):
		switch *m.DvProfile {
		case 5:
			return enums.VideoRangeHDR, enums.VideoRangeTypeDOVI
		case 8:
			switch *m.DvBlSignalCompatibilityId {
			case 1:
				return enums.VideoRangeHDR, enums.VideoRangeTypeDOVIWithHDR10
			case 4:
				return enums.VideoRangeHDR, enums.VideoRangeTypeDOVIWithHLG
			case 2:
				return enums.VideoRangeSDR, enums.VideoRangeTypeDOVIWithSDR
			case 6:
				return enums.VideoRangeHDR, enums.VideoRangeTypeDOVIWithHDR10
			default:
				return enums.VideoRangeSDR, enums.VideoRangeTypeSDR
			}
		case 7:
			return enums.VideoRangeHDR, enums.VideoRangeTypeHDR10
		default:
			return enums.VideoRangeSDR, enums.VideoRangeTypeSDR
		}
	case strings.EqualFold(m.ColorTransfer, "smpte2084"):
		return enums.VideoRangeHDR, enums.VideoRangeTypeHDR10
	case strings.EqualFold(m.ColorTransfer, "arib-std-b67"):
		return enums.VideoRangeHDR, enums.VideoRangeTypeHLG
	default:
		return enums.VideoRangeSDR, enums.VideoRangeTypeSDR
	}
}

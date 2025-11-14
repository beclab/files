package probing

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"files/pkg/media/jellyfin/data/enums"
	"files/pkg/media/mediabrowser/model/dto"
	"files/pkg/media/mediabrowser/model/entities"
	"files/pkg/media/mediabrowser/model/mediainfo"
	"files/pkg/media/mediabrowser/model/mediainfo/mediaprotocol"
	"files/pkg/media/mediabrowser/model/mediainfo/transportstreamtimestamp"
	"files/pkg/media/utils"

	"k8s.io/klog/v2"
)

type ProbeResultNormalizer struct {
	MaxSubtitleDescriptionExtractionLength int
	ArtistReplaceValue                     string
	nameDelimiters                         []rune
	webmVideoCodecs                        []string
	webmAudioCodecs                        []string
	logger                                 *utils.Logger
	//	localization                           ILocalizationManager
	splitWhiteList []string
}

func NewProbeResultNormalizer(logger *utils.Logger /*, localization ILocalizationManager*/) *ProbeResultNormalizer {
	return &ProbeResultNormalizer{
		MaxSubtitleDescriptionExtractionLength: 100,
		ArtistReplaceValue:                     " | ",
		nameDelimiters:                         []rune{'/', '|', ';', '\\'},
		webmVideoCodecs:                        []string{"av1", "vp8", "vp9"},
		webmAudioCodecs:                        []string{"opus", "vorbis"},
		logger:                                 logger,
		//		localization:                           localization,
	}
}

func (p *ProbeResultNormalizer) splitWhitelist() []string {
	if p.splitWhiteList != nil {
		p.splitWhiteList = []string{
			"AC/DC",
			"A/T/O/S",
			"As/Hi Soundworks",
			"Au/Ra",
			"Bremer/McCoy",
			"b/bqスタヂオ",
			"DOV/S",
			"DJ'TEKINA//SOMETHING",
			"IX/ON",
			"J-CORE SLi//CER",
			"M(a/u)SH",
			"Kaoru/Brilliance",
			"signum/ii",
			"Richiter(LORB/DUGEM DI BARAT)",
			"이달의 소녀 1/3",
			"R!N / Gemie",
			"LOONA 1/3",
			"LOONA / yyxy",
			"LOONA / ODD EYE CIRCLE",
			"K/DA",
			"22/7",
			"諭吉佳作/men",
			"//dARTH nULL",
			"Phantom/Ghost",
			"She/Her/Hers",
			"5/8erl in Ehr'n",
			"Smith/Kotzen",
			"We;Na",
			"LSR/CITY",
		}
	}

	return p.splitWhiteList
}

func (p *ProbeResultNormalizer) GetMediaInfo(data *InternalMediaInfoResult, videoType *entities.VideoType, isAudio bool, path string, protocol mediaprotocol.MediaProtocol) *mediainfo.MediaInfo {
	info := &mediainfo.MediaInfo{
		MediaSourceInfo: dto.MediaSourceInfo{
			Path:      path,
			Protocol:  protocol,
			VideoType: videoType,
		},
	}

	NormalizeFFProbeResult(data)
	p.setSize(data, info)

	var internalStreams []*MediaStreamInfo
	if data.Streams != nil {
		internalStreams = data.Streams
	} else {
		internalStreams = []*MediaStreamInfo{}
	}

	info.MediaStreams = make([]entities.MediaStream, 0, len(internalStreams))
	for _, s := range internalStreams {
		mediaStream := p.getMediaStream(isAudio, *s, data.Format)
		if mediaStream != nil {
			// Drop subtitle streams if we don't know the codec because it will just cause failures if we don't know how to handle them
			if mediaStream.Type != entities.MediaStreamTypeSubtitle || mediaStream.Codec != "" {
				info.MediaStreams = append(info.MediaStreams, *mediaStream)
			}
		}
	}

	info.MediaAttachments = make([]entities.MediaAttachment, 0, len(internalStreams))
	for _, s := range internalStreams {
		attachment := p.getMediaAttachment(*s)
		if attachment != nil {
			info.MediaAttachments = append(info.MediaAttachments, *attachment)
		}
	}

	if data.Format != nil {
		info.Container = p.normalizeFormat(data.Format.FormatName, info.MediaStreams)

		if bitRate, err := strconv.Atoi(data.Format.BitRate); err == nil {
			info.Bitrate = &bitRate
		}
	}

	tags := make(map[string]string)
	var tagStream *MediaStreamInfo
	var tagStreamType CodecType
	if isAudio {
		tagStreamType = Audio
	} else {
		tagStreamType = Video
	}

	for _, s := range data.Streams {
		if s.CodecType == tagStreamType {
			tagStream = s
			break
		}
	}

	if tagStream != nil && tagStream.Tags != nil {
		for key, value := range tagStream.Tags {
			tags[key] = value
		}
	}

	if data.Format != nil && data.Format.Tags != nil {
		for key, value := range data.Format.Tags {
			tags[key] = value
		}
	}

	p.fetchGenres(info, tags)

	info.Name = GetFirstNotNullNorWhiteSpaceValue(tags, "title", "title-eng")
	info.ForcedSortName = GetFirstNotNullNorWhiteSpaceValue(tags, "sort_name", "title-sort", "titlesort")
	info.Overview = GetFirstNotNullNorWhiteSpaceValue(tags, "synopsis", "description", "desc")

	info.IndexNumber = GetDictionaryNumericValue(tags, "episode_sort")
	info.ParentIndexNumber = GetDictionaryNumericValue(tags, "season_number")
	info.ShowName = tags["show_name"]
	info.ProductionYear = GetDictionaryNumericValue(tags, "date")

	// Several different forms of retail/premiere date
	info.PremiereDate = func() *time.Time {
		var date *time.Time
		date = GetDictionaryDateTime(tags, "originaldate")
		if date != nil {
			return date
		}
		date = GetDictionaryDateTime(tags, "retaildate")
		if date != nil {
			return date
		}
		date = GetDictionaryDateTime(tags, "retail date")
		if date != nil {
			return date
		}
		date = GetDictionaryDateTime(tags, "retail_date")
		if date != nil {
			return date
		}
		date = GetDictionaryDateTime(tags, "date_released")
		if date != nil {
			return date
		}
		date = GetDictionaryDateTime(tags, "date")
		if date != nil {
			return date
		}
		date = GetDictionaryDateTime(tags, "creation_time")
		return date
	}()

	// Set common metadata for music (audio) and music videos (video)
	info.Album = tags["album"]

	if artists, ok := tags["artists"]; ok && artists != "" {
		info.Artists = p.splitDistinctArtists(artists, []rune{'/', ';'}, false)
	} else {
		artist := GetFirstNotNullNorWhiteSpaceValue(tags, "artist")
		if artist != "" {
			info.Artists = p.splitDistinctArtists(artist, p.nameDelimiters, true)
		} else {
			info.Artists = []string{}
		}
	}

	// Guess ProductionYear from PremiereDate if missing
	if info.ProductionYear != nil && info.PremiereDate != nil {
		*info.ProductionYear = info.PremiereDate.Year()
	}

	// Set mediaType-specific metadata
	if isAudio {
		p.setAudioRuntimeTicks(data, info)
		p.setAudioInfoFromTags(info, tags)
	} else {
		p.fetchStudios(info, tags, "copyright")

		iTunExtc := GetFirstNotNullNorWhiteSpaceValue(tags, "iTunEXTC")
		if iTunExtc != "" {
			parts := strings.Split(iTunExtc, "|")
			if len(parts) > 1 {
				info.OfficialRating = parts[1]
				if len(parts) > 3 {
					info.OfficialRatingDescription = parts[3]
				}
			}
		}

		iTunXml := GetFirstNotNullNorWhiteSpaceValue(tags, "iTunMOVI")
		if iTunXml != "" {
			p.fetchFromItunesInfo(iTunXml, info)
		}

		if data.Format != nil && data.Format.Duration != "" {
			duration, err := strconv.ParseFloat(data.Format.Duration, 64)
			klog.Infoln(data.Format.Duration)
			if err == nil {
				runTimeTicks := int64(duration*1_000_000_000) / 100
				info.RunTimeTicks = &runTimeTicks
			}
		}

		p.fetchWtvInfo(info, data)

		if data.Chapters != nil {
			info.Chapters = make([]*entities.ChapterInfo, len(data.Chapters))
			for i, chapter := range data.Chapters {
				info.Chapters[i] = GetChapterInfo(chapter)
			}
		}

		p.extractTimestamp(info)

		if stereoMode, ok := tags["stereo_mode"]; ok && strings.EqualFold(stereoMode, "left_right") {
			*info.Video3DFormat = entities.FullSideBySide
		}

		for _, mediaStream := range info.MediaStreams {
			if mediaStream.Type == entities.MediaStreamTypeAudio && mediaStream.BitRate != nil {
				mediaStream.BitRate = p.getEstimatedAudioBitrate(mediaStream.Codec, mediaStream.Channels)
			}
		}

		var videoStreamsBitrate int64
		for _, mediaStream := range info.MediaStreams {
			if mediaStream.Type == entities.MediaStreamTypeVideo {
				if mediaStream.BitRate != nil {
					videoStreamsBitrate += int64(*mediaStream.BitRate)
				}
			}
		}

		// If ffprobe reported the container bitrate as being the same as the video stream bitrate, then it's wrong
		if (info.Bitrate == nil && videoStreamsBitrate == 0) || (info.Bitrate != nil && videoStreamsBitrate == int64(*info.Bitrate)) {
			info.InferTotalBitrate(true)
		}
	}

	return info
}

func GetFirstNotNullNorWhiteSpaceValue(tags map[string]string, args ...string) string {
	return ""
}

func (p *ProbeResultNormalizer) normalizeFormat(format string, mediaStreams []entities.MediaStream) string {
	if len(format) == 0 || strings.TrimSpace(format) == "" {
		return ""
	}

	splitFormat := strings.Split(format, ",")
	for i := range splitFormat {
		if strings.EqualFold(splitFormat[i], "mpegvideo") {
			// Handle MPEG-1 container
			splitFormat[i] = "mpeg"
		} else if strings.EqualFold(splitFormat[i], "mpegts") {
			// Handle MPEG-TS container
			splitFormat[i] = "ts"
		} else if strings.EqualFold(splitFormat[i], "matroska") {
			// Handle matroska container
			splitFormat[i] = "mkv"
		} else if strings.EqualFold(splitFormat[i], "webm") {
			// Handle WebM
			// Limit WebM to supported codecs
			if containsUnsupportedCodec(mediaStreams, "webm") {
				splitFormat[i] = ""
			}
		}
	}

	return strings.Join(filterEmptyStrings(splitFormat), ",")
}

func containsUnsupportedCodec(mediaStreams []entities.MediaStream, container string) bool {
	var webmVideoCodecs = []string{"vp8", "vp9", "av1"}
	var webmAudioCodecs = []string{"opus", "vorbis"}

	for _, stream := range mediaStreams {
		if stream.Type == entities.MediaStreamTypeVideo {
			if !contains(webmVideoCodecs, stream.Codec) {
				return true
			}
		} else if stream.Type == entities.MediaStreamTypeAudio {
			if !contains(webmAudioCodecs, stream.Codec) {
				return true
			}
		}
	}
	return false
}

func contains(slice []string, str string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, str) {
			return true
		}
	}
	return false
}

func filterEmptyStrings(slice []string) []string {
	var result []string
	for _, s := range slice {
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func (p *ProbeResultNormalizer) getEstimatedAudioBitrate(codec string, channels *int) *int {
	if channels == nil {
		return nil
	}

	channelsValue := *channels

	switch {
	case strings.EqualFold(codec, "aac") || strings.EqualFold(codec, "mp3"):
		switch {
		case channelsValue <= 2:
			return toPtr(192000)
		case channelsValue >= 5:
			return toPtr(320000)
		}
	case strings.EqualFold(codec, "ac3") || strings.EqualFold(codec, "eac3"):
		switch {
		case channelsValue <= 2:
			return toPtr(192000)
		case channelsValue >= 5:
			return toPtr(640000)
		}
	case strings.EqualFold(codec, "flac") || strings.EqualFold(codec, "alac"):
		switch {
		case channelsValue <= 2:
			return toPtr(960000)
		case channelsValue >= 5:
			return toPtr(2880000)
		}
	}

	return nil
}

func toPtr[T any](value T) *T {
	return &value
}

func (p *ProbeResultNormalizer) fetchFromItunesInfo(xmlData string, info *mediainfo.MediaInfo) {
	// Make things simpler and strip out the dtd
	plistIndex := strings.Index(strings.ToLower(xmlData), "<plist")
	if plistIndex != -1 {
		xmlData = xmlData[plistIndex:]
	}

	xmlData = "<?xml version=\"1.0\"?>" + xmlData

	reader := bytes.NewReader([]byte(xmlData))
	decoder := xml.NewDecoder(reader)

	for {
		t, _ := decoder.Token()
		if t == nil {
			break
		}

		switch se := t.(type) {
		case xml.StartElement:
			switch se.Name.Local {
			case "dict":
				//				if decoder.IsEmpty() {
				if se.Name.Space == "" && len(se.Attr) == 0 {
					decoder.Skip()
					continue
				}
				p.readFromDictNode(decoder, info)
			default:
				decoder.Skip()
			}
		default:
			// ignore other token types
		}
	}
}

func (p *ProbeResultNormalizer) readFromDictNode(decoder *xml.Decoder, info *mediainfo.MediaInfo) {
	/*
			var currentKey string
			var pairs []dto.NameValuePair

			for {
				t, _ := decoder.Token()
				if t == nil {
					break
				}

				switch se := t.(type) {
				case xml.StartElement:
					switch se.Name.Local {
					case "key":
						if currentKey != "" {
							p.processPairs(currentKey, pairs, info)
						}
						decoder.DecodeElement(&currentKey, &se)
						pairs = nil
					case "string":
						var value string
						decoder.DecodeElement(&value, &se)
						if value != "" {
							pairs = append(pairs, dto.NameValuePair{
								Name:  value,
								Value: value,
							})
						}
					case "array":
		//				if reader.IsEmptyElement {
						if t.Self {
							reader.Skip()
							continue
						}
						if currentKey != "" {
							pairs = append(pairs, readValueArray(reader)...)
						}
					default:
						decoder.Skip()
					}
				case xml.EndElement:
					if se.Name.Local == "dict" {
						if currentKey != "" {
							p.processPairs(currentKey, pairs, info)
						}
						return
					}
				}
			}
	*/
}

func (p *ProbeResultNormalizer) readValueArray(decoder *xml.Decoder) []dto.NameValuePair {
	var pairs []dto.NameValuePair

	/*
		for {
			t, _ := decoder.Token()
			if t == nil {
				break
			}

			switch se := t.(type) {
			case xml.StartElement:
				switch se.Name.Local {
				case "dict":
					dict := p.getNameValuePair(decoder)
					if dict != nil {
						pairs = append(pairs, *dict)
					}
				default:
					decoder.Skip()
				}
			case xml.EndElement:
				if se.Name.Local == "array" {
					return pairs
				}
			}
		}
	*/

	return pairs
}

func (p *ProbeResultNormalizer) processPairs(key string, pairs []dto.NameValuePair, info *mediainfo.MediaInfo) {
	var peoples []dto.BaseItemPerson

	switch strings.ToLower(key) {
	case "studio":
		studios := make([]string, 0, len(pairs))
		for _, pair := range pairs {
			if pair.Value != "" {
				studios = append(studios, pair.Value)
			}
		}
		info.Studios = uniqueStrings(studios)
	case "screenwriters":
		for _, pair := range pairs {
			peoples = append(peoples, dto.BaseItemPerson{
				Name: pair.Value,
				Type: enums.Writer,
			})
		}
	case "producers":
		for _, pair := range pairs {
			peoples = append(peoples, dto.BaseItemPerson{
				Name: pair.Value,
				Type: enums.Producer,
			})
		}
	case "directors":
		for _, pair := range pairs {
			peoples = append(peoples, dto.BaseItemPerson{
				Name: pair.Value,
				Type: enums.Director,
			})
		}
	}

	// info.People = peoples
}

func uniqueStrings(strs []string) []string {
	uniqueMap := make(map[string]struct{}, len(strs))
	for _, str := range strs {
		uniqueMap[str] = struct{}{}
	}
	unique := make([]string, 0, len(uniqueMap))
	for str := range uniqueMap {
		unique = append(unique, str)
	}
	return unique
}

/*
func (p *ProbeResultNormalizer) getNameValuePair(reader *xml.Decoder) *dto.NameValuePair {
	var name, value string
	var pair *dto.NameValuePair

	for {
		t, err := reader.Token()
		if err != nil {
			return nil
		}

		switch el := t.(type) {
		case xml.StartElement:
			switch el.Name.Local {
			case "key":
				name, err = reader.ReadElementString(el)
				if err != nil {
					return nil
				}
			case "string":
				value, err = reader.ReadElementString(el)
				if err != nil {
					return nil
				}
			default:
				reader.Skip()
			}
		case xml.EndElement:
			if el.Name.Local == "NameValuePair" {
				if name == "" || value == "" {
					return nil
				}
				pair = &dto.NameValuePair{
					Name:  name,
					Value: value,
				}
				return pair
			}
		}
	}
}
*/

func (p *ProbeResultNormalizer) normalizeSubtitleCodec(codec string) string {
	switch strings.ToLower(codec) {
	case "dvb_subtitle":
		return "DVBSUB"
	case "dvb_teletext":
		return "DVBTXT"
	case "dvd_subtitle":
		return "DVDSUB" // .sub+.idx
	case "hdmv_pgs_subtitle":
		return "PGSSUB" // .sup
	default:
		return codec
	}
}

func (p *ProbeResultNormalizer) getMediaAttachment(streamInfo MediaStreamInfo) *entities.MediaAttachment {
	if streamInfo.CodecType != Attachment && (streamInfo.Disposition == nil || streamInfo.Disposition["attached_pic"] != 1) {
		return nil
	}

	attachment := &entities.MediaAttachment{
		Codec: streamInfo.CodecName,
		Index: streamInfo.Index,
	}

	if streamInfo.CodecTagString != "" {
		attachment.CodecTag = streamInfo.CodecTagString
	}

	if streamInfo.Tags != nil {
		attachment.FileName = p.getDictionaryValue(streamInfo.Tags, "filename")
		attachment.MimeType = p.getDictionaryValue(streamInfo.Tags, "mimetype")
		attachment.Comment = p.getDictionaryValue(streamInfo.Tags, "comment")
	}

	return attachment
}

func (p *ProbeResultNormalizer) getMediaStream(isAudio bool, streamInfo MediaStreamInfo, formatInfo *MediaFormatInfo) *entities.MediaStream {
	// These are mp4 chapters
	if strings.EqualFold(streamInfo.CodecName, "mov_text") {
		// Edit: but these are also sometimes subtitles?
		// return nil
	}

	level := float64(streamInfo.Level)
	stream := &entities.MediaStream{
		Codec:         streamInfo.CodecName,
		Profile:       streamInfo.Profile,
		Level:         &level,
		Index:         streamInfo.Index,
		PixelFormat:   streamInfo.PixelFormat,
		NalLengthSize: streamInfo.NalLengthSize,
		TimeBase:      streamInfo.TimeBase,
		CodecTimeBase: streamInfo.CodecTimeBase,
		IsAVC:         &streamInfo.IsAvc,
	}

	// Filter out junk
	if streamInfo.CodecTagString != "" && !strings.Contains(streamInfo.CodecTagString, "[0]") {
		stream.CodecTag = streamInfo.CodecTagString
	}

	if streamInfo.Tags != nil {
		stream.Language = p.getDictionaryValue(streamInfo.Tags, "language")
		stream.Comment = p.getDictionaryValue(streamInfo.Tags, "comment")
		stream.Title = p.getDictionaryValue(streamInfo.Tags, "title")
	}

	if streamInfo.CodecType == Audio {
		stream.Type = entities.MediaStreamTypeAudio
		stream.Channels = &streamInfo.Channels

		sampleRate, err := strconv.Atoi(streamInfo.SampleRate)
		if err == nil {
			stream.SampleRate = &sampleRate
		}

		stream.ChannelLayout = p.parseChannelLayout(streamInfo.ChannelLayout)

		if streamInfo.BitsPerSample > 0 {
			stream.BitDepth = &streamInfo.BitsPerSample
		} else if streamInfo.BitsPerRawSample > 0 {
			stream.BitDepth = &streamInfo.BitsPerRawSample
		}

		if stream.Title == "" {
			// mp4 missing track title workaround: fall back to handler_name if populated and not the default "SoundHandler"
			handlerName := p.getDictionaryValue(streamInfo.Tags, "handler_name")
			if handlerName != "" && !strings.EqualFold(handlerName, "SoundHandler") {
				stream.Title = handlerName
			}
		}
	} else if streamInfo.CodecType == Subtitle {
		stream.Type = entities.MediaStreamTypeSubtitle
		stream.Codec = p.normalizeSubtitleCodec(stream.Codec)
		/* compile
		stream.LocalizedUndefined = _localization.GetLocalizedString("Undefined")
		stream.LocalizedDefault = _localization.GetLocalizedString("Default")
		stream.LocalizedForced = _localization.GetLocalizedString("Forced")
		stream.LocalizedExternal = _localization.GetLocalizedString("External")
		stream.LocalizedHearingImpaired = _localization.GetLocalizedString("HearingImpaired")
		*/

		// Graphical subtitle may have width and height info
		stream.Width = &streamInfo.Width
		stream.Height = &streamInfo.Height

		if stream.Title == "" {
			// mp4 missing track title workaround: fall back to handler_name if populated and not the default "SubtitleHandler"
			handlerName := p.getDictionaryValue(streamInfo.Tags, "handler_name")
			if handlerName != "" && !strings.EqualFold(handlerName, "SubtitleHandler") {
				stream.Title = handlerName
			}
		}
	} else if streamInfo.CodecType == Video {
		stream.AverageFrameRate = GetFrameRate([]byte(streamInfo.AverageFrameRate))
		stream.RealFrameRate = GetFrameRate([]byte(streamInfo.RFrameRate))

		stream.IsInterlaced = streamInfo.FieldOrder != "" && !strings.EqualFold(streamInfo.FieldOrder, "progressive")

		if isAudio ||
			strings.EqualFold(stream.Codec, "bmp") ||
			strings.EqualFold(stream.Codec, "gif") ||
			strings.EqualFold(stream.Codec, "png") ||
			strings.EqualFold(stream.Codec, "webp") {
			stream.Type = entities.MediaStreamTypeEmbeddedImage
		} else if strings.EqualFold(stream.Codec, "mjpeg") {
			if streamInfo.CodecTag != "" {
				stream.Type = entities.MediaStreamTypeVideo
			} else {
				stream.Type = entities.MediaStreamTypeEmbeddedImage
			}
		} else {
			stream.Type = entities.MediaStreamTypeVideo
		}

		stream.Width = &streamInfo.Width
		stream.Height = &streamInfo.Height
		stream.AspectRatio = p.getAspectRatio(streamInfo)

		if streamInfo.BitsPerSample > 0 {
			stream.BitDepth = &streamInfo.BitsPerSample
		} else if streamInfo.BitsPerRawSample > 0 {
			stream.BitDepth = &streamInfo.BitsPerRawSample
		}

		if stream.BitDepth == nil {
			var bitDepth int
			if streamInfo.PixelFormat != "" {
				if strings.EqualFold(streamInfo.PixelFormat, "yuv420p") ||
					strings.EqualFold(streamInfo.PixelFormat, "yuv444p") {
					bitDepth = 8
					stream.BitDepth = &bitDepth
				} else if strings.EqualFold(streamInfo.PixelFormat, "yuv420p10le") ||
					strings.EqualFold(streamInfo.PixelFormat, "yuv444p10le") {
					bitDepth = 10
					stream.BitDepth = &bitDepth
				} else if strings.EqualFold(streamInfo.PixelFormat, "yuv420p12le") ||
					strings.EqualFold(streamInfo.PixelFormat, "yuv444p12le") {
					bitDepth = 12
					stream.BitDepth = &bitDepth
				}
			}
		}

		isAnamorphic := strings.EqualFold(streamInfo.SampleAspectRatio, "0:1")
		stream.IsAnamorphic = &isAnamorphic

		if streamInfo.Refs > 0 {
			stream.RefFrames = &streamInfo.Refs
		}

		if streamInfo.ColorRange != "" {
			stream.ColorRange = streamInfo.ColorRange
		}

		if streamInfo.ColorSpace != "" {
			stream.ColorSpace = streamInfo.ColorSpace
		}

		if streamInfo.ColorTransfer != "" {
			stream.ColorTransfer = streamInfo.ColorTransfer
		}

		if streamInfo.ColorPrimaries != "" {
			stream.ColorPrimaries = streamInfo.ColorPrimaries
		}

		for _, data := range streamInfo.SideDataList {
			if strings.EqualFold(*data.SideDataType, "DOVI configuration record") {
				stream.DvVersionMajor = data.DvVersionMajor
				stream.DvVersionMinor = data.DvVersionMinor
				stream.DvProfile = data.DvProfile
				stream.DvLevel = data.DvLevel
				stream.RpuPresentFlag = data.RpuPresentFlag
				stream.ElPresentFlag = data.ElPresentFlag
				stream.BlPresentFlag = data.BlPresentFlag
				stream.DvBlSignalCompatibilityId = data.DvBlSignalCompatibilityId
				break
			}
		}
	} else if streamInfo.CodecType == Data {
		stream.Type = entities.MediaStreamTypeData
	} else {
		return nil
	}

	// Get stream bitrate
	bitrate := 0
	if streamInfo.BitRate != "" {
		klog.Infoln("streaminfo bitrate:", streamInfo.BitRate)
		bitrate, _ = strconv.Atoi(streamInfo.BitRate)
	}

	// The bitrate info of FLAC musics and some videos is included in formatInfo.
	if bitrate == 0 && formatInfo != nil {
		if stream.Type == entities.MediaStreamTypeVideo || (isAudio && stream.Type == entities.MediaStreamTypeAudio) {
			// If the stream info doesn't have a bitrate get the value from the media format info
			if formatInfo.BitRate != "" {
				bitrate, _ = strconv.Atoi(formatInfo.BitRate)
			}
		}
	}

	if bitrate > 0 {
		stream.BitRate = &bitrate
		klog.Infoln("stream bitrate: ", *stream.BitRate)
	}

	// Extract bitrate info from tag "BPS" if possible.
	if stream.BitRate != nil && (streamInfo.CodecType == Audio || streamInfo.CodecType == Video) {
		bps := p.getBPSFromTags(&streamInfo)
		if bps != nil && *bps > 0 {
			stream.BitRate = bps
		} else {
			// Get average bitrate info from tag "NUMBER_OF_BYTES" and "DURATION" if possible.
			durationInSeconds := p.getRuntimeSecondsFromTags(streamInfo)
			bytes := p.getNumberOfBytesFromTags(streamInfo)
			if durationInSeconds != nil && bytes != nil {
				tmp := int(float64(*bytes*8) / *durationInSeconds)
				bps = &tmp
				if *bps > 0 {
					stream.BitRate = bps
				}
			}
		}
	}

	if streamInfo.Disposition != nil {
		if streamInfo.Disposition["default"] == 1 {
			stream.IsDefault = true
		}
		if streamInfo.Disposition["forced"] == 1 {
			stream.IsForced = true
		}
		if streamInfo.Disposition["hearing_impaired"] == 1 {
			stream.IsHearingImpaired = true
		}
	}

	p.normalizeStreamTitle(stream)

	//klog.Infoln("7777777888888888:", *stream.BitRate)
	return stream
}

func (p *ProbeResultNormalizer) normalizeStreamTitle(stream *entities.MediaStream) {
	if strings.EqualFold(stream.Title, "cc") || stream.Type == entities.MediaStreamTypeEmbeddedImage {
		stream.Title = ""
	}
}

// GetDictionaryValue gets a string from an FFProbeResult tags dictionary.
func (p *ProbeResultNormalizer) getDictionaryValue(tags map[string]string, key string) string {
	if tags == nil {
		return ""
	}

	val, ok := tags[key]
	if !ok {
		return ""
	}

	return val
}

func (p *ProbeResultNormalizer) parseChannelLayout(input string) string {
	if input == "" {
		return ""
	}

	i := strings.IndexByte(input, '(')
	if i == -1 {
		return input
	}

	return input[:i]
}

func (p *ProbeResultNormalizer) getAspectRatio(info MediaStreamInfo) string {
	original := info.DisplayAspectRatio

	parts := strings.Split(original, ":")
	if len(parts) == 2 {
		width, _ := strconv.Atoi(parts[0])
		height, _ := strconv.Atoi(parts[1])
		if width > 0 && height > 0 {
			ratio := float64(width) / float64(height)
			if p.isClose(ratio, 1.777777778, 0.03) {
				return "16:9"
			}
			if p.isClose(ratio, 1.3333333333, 0.05) {
				return "4:3"
			}
			if p.isClose(ratio, 1.41, 0.01) {
				return "1.41:1"
			}
			if p.isClose(ratio, 1.5, 0.01) {
				return "1.5:1"
			}
			if p.isClose(ratio, 1.6, 0.01) {
				return "1.6:1"
			}
			if p.isClose(ratio, 1.66666666667, 0.01) {
				return "5:3"
			}
			if p.isClose(ratio, 1.85, 0.02) {
				return "1.85:1"
			}
			if p.isClose(ratio, 2.35, 0.025) {
				return "2.35:1"
			}
			if p.isClose(ratio, 2.4, 0.025) {
				return "2.40:1"
			}
		}
	} else {
		width := info.Width
		height := info.Height
		if width > 0 && height > 0 {
			ratio := float64(width) / float64(height)
			if p.isClose(ratio, 1.777777778, 0.03) {
				return "16:9"
			}
			if p.isClose(ratio, 1.3333333333, 0.05) {
				return "4:3"
			}
			if p.isClose(ratio, 1.41, 0.01) {
				return "1.41:1"
			}
			if p.isClose(ratio, 1.5, 0.01) {
				return "1.5:1"
			}
			if p.isClose(ratio, 1.6, 0.01) {
				return "1.6:1"
			}
			if p.isClose(ratio, 1.66666666667, 0.01) {
				return "5:3"
			}
			if p.isClose(ratio, 1.85, 0.02) {
				return "1.85:1"
			}
			if p.isClose(ratio, 2.35, 0.025) {
				return "2.35:1"
			}
			if p.isClose(ratio, 2.4, 0.025) {
				return "2.40:1"
			}
		}
	}

	return original
}

func (p *ProbeResultNormalizer) isClose(d1, d2, variance float64) bool {
	return math.Abs(d1-d2) <= variance
}

func GetFrameRate(value []byte) *float32 {
	if len(value) == 0 {
		return nil
	}

	idx := bytes.IndexByte(value, '/')
	if idx == -1 {
		return nil
	}

	var dividend, divisor float32
	if _, err := fmt.Sscanf(string(value[:idx]), "%f", &dividend); err != nil {
		return nil
	}
	if _, err := fmt.Sscanf(string(value[idx+1:]), "%f", &divisor); err != nil {
		return nil
	}

	if divisor == 0 {
		return nil
	}
	frameRate := dividend / divisor
	return &frameRate
}

// SetAudioRuntimeTicks sets the audio runtime ticks in the MediaInfo data.
func (p *ProbeResultNormalizer) setAudioRuntimeTicks(result *InternalMediaInfoResult, data *mediainfo.MediaInfo) {
	// Get the first audio stream
	var stream *MediaStreamInfo
	for _, s := range result.Streams {
		if s.CodecType == Audio {
			stream = s
			break
		}
	}
	if stream == nil {
		return
	}

	// Get duration from stream properties
	duration := stream.Duration
	if duration == "" {
		// If it's not there, get it from format properties
		duration = result.Format.Duration
	}

	// If we got something, parse it
	if duration != "" {
		d, err := strconv.ParseFloat(duration, 64)
		if err == nil {
			runTimeTicks := int64(d*float64(time.Second)) / 100
			data.RunTimeTicks = &runTimeTicks
		}
	}
}

// GetBPSFromTags gets the BPS (bits per second) value from the stream tags.
func (p *ProbeResultNormalizer) getBPSFromTags(streamInfo *MediaStreamInfo) *int {
	if streamInfo == nil || streamInfo.Tags == nil {
		return nil
	}

	bps := p.getDictionaryValue(streamInfo.Tags, "BPS-eng")
	if bps == "" {
		bps = p.getDictionaryValue(streamInfo.Tags, "BPS")
	}

	var parsedBps int
	if _, err := fmt.Sscanf(bps, "%d", &parsedBps); err == nil {
		return &parsedBps
	}

	return nil
}
func (p *ProbeResultNormalizer) getRuntimeSecondsFromTags(streamInfo MediaStreamInfo) *float64 {
	if streamInfo.Tags == nil {
		return nil
	}

	duration := p.getDictionaryValue(streamInfo.Tags, "DURATION-eng")
	if duration == "" {
		duration = p.getDictionaryValue(streamInfo.Tags, "DURATION")
	}
	if duration != "" {
		parsedDuration, err := time.ParseDuration(duration)
		if err == nil {
			seconds := parsedDuration.Seconds()
			return &seconds
		}
	}

	return nil
}

func (p *ProbeResultNormalizer) getNumberOfBytesFromTags(streamInfo MediaStreamInfo) *int64 {
	if streamInfo.Tags == nil {
		return nil
	}

	numberOfBytes := p.getDictionaryValue(streamInfo.Tags, "NUMBER_OF_BYTES-eng")
	if numberOfBytes == "" {
		numberOfBytes = p.getDictionaryValue(streamInfo.Tags, "NUMBER_OF_BYTES")
	}
	if numberOfBytes != "" {
		var parsedBytes int64
		_, err := fmt.Sscanf(numberOfBytes, "%d", &parsedBytes)
		if err == nil {
			return &parsedBytes
		}
	}

	return nil
}

func (p *ProbeResultNormalizer) setSize(data *InternalMediaInfoResult, info *mediainfo.MediaInfo) {
	if data.Format == nil {
		return
	}

	if data.Format.Size != "" {
		size, err := strconv.ParseInt(data.Format.Size, 10, 64)
		if err == nil {
			info.Size = &size
		}
	} else {
		info.Size = nil
	}
}

func (p *ProbeResultNormalizer) setAudioInfoFromTags(audio *mediainfo.MediaInfo, tags map[string]string) {
	people := make([]dto.BaseItemPerson, 0)

	if composer, ok := tags["composer"]; ok && composer != "" {
		for _, person := range p.split(composer, false) {
			people = append(people, dto.BaseItemPerson{
				Name: person,
				Type: enums.Composer,
			})
		}
	}

	if conductor, ok := tags["conductor"]; ok && conductor != "" {
		for _, person := range p.split(conductor, false) {
			people = append(people, dto.BaseItemPerson{
				Name: person,
				Type: enums.Conductor,
			})
		}
	}

	if lyricist, ok := tags["lyricist"]; ok && lyricist != "" {
		for _, person := range p.split(lyricist, false) {
			people = append(people, dto.BaseItemPerson{
				Name: person,
				Type: enums.Lyricist,
			})
		}
	}

	if performer, ok := tags["performer"]; ok && performer != "" {
		for _, person := range p.split(performer, false) {
			match := PerformerRegex().FindStringSubmatch(person)
			if len(match) > 0 {
				people = append(people, dto.BaseItemPerson{
					Name: match[1],
					Type: enums.Actor,
					Role: strings.Title(match[2]),
				})
			}
		}
	}

	if writer, ok := tags["writer"]; ok && writer != "" {
		for _, person := range p.split(writer, false) {
			people = append(people, dto.BaseItemPerson{
				Name: person,
				Type: enums.Writer,
			})
		}
	}

	if arranger, ok := tags["arranger"]; ok && arranger != "" {
		for _, person := range p.split(arranger, false) {
			people = append(people, dto.BaseItemPerson{
				Name: person,
				Type: enums.Arranger,
			})
		}
	}

	if engineer, ok := tags["engineer"]; ok && engineer != "" {
		for _, person := range p.split(engineer, false) {
			people = append(people, dto.BaseItemPerson{
				Name: person,
				Type: enums.Engineer,
			})
		}
	}

	if mixer, ok := tags["mixer"]; ok && mixer != "" {
		for _, person := range p.split(mixer, false) {
			people = append(people, dto.BaseItemPerson{
				Name: person,
				Type: enums.Mixer,
			})
		}
	}

	if remixer, ok := tags["remixer"]; ok && remixer != "" {
		for _, person := range p.split(remixer, false) {
			people = append(people, dto.BaseItemPerson{
				Name: person,
				Type: enums.Remixer,
			})
		}
	}

	//	audio.People = people

	albumArtist := GetFirstNotNullNorWhiteSpaceValue(tags, "albumartist", "album artist", "album_artist")
	audio.AlbumArtists = p.splitDistinctArtists(albumArtist, p.nameDelimiters, true)

	if len(audio.AlbumArtists) == 0 {
		audio.AlbumArtists = audio.Artists
	}

	audio.IndexNumber = GetDictionaryTrackOrDiscNumber(tags, "track")
	audio.ParentIndexNumber = GetDictionaryTrackOrDiscNumber(tags, "disc")

	p.fetchStudios(audio, tags, "organization")
	p.fetchStudios(audio, tags, "ensemble")
	p.fetchStudios(audio, tags, "publisher")
	p.fetchStudios(audio, tags, "label")

	/*
			mb := p.getMultipleMusicBrainzId(&tags["MusicBrainz Album Artist Id"])
			if mb == nil {
				mb = p.getMultipleMusicBrainzId(tags["MUSICBRAINZ_ALBUMARTISTID"])
			}
		//	audio.SetProviderId(MetadataProvider_MusicBrainzAlbumArtist, mb)

			mb = p.getMultipleMusicBrainzId(tags["MusicBrainz Artist Id"])
			if mb == nil {
				mb = p.getMultipleMusicBrainzId(tags["MUSICBRAINZ_ARTISTID"])
			}
		//	audio.SetProviderId(MetadataProvider_MusicBrainzArtist, mb)

			mb = p.getMultipleMusicBrainzId(tags["MusicBrainz Album Id"])
			if mb == nil {
				mb = p.getMultipleMusicBrainzId(tags["MUSICBRAINZ_ALBUMID"])
			}
		//	audio.SetProviderId(MetadataProvider_MusicBrainzAlbum, mbAlbumId)

			mb = p.getMultipleMusicBrainzId(tags["MusicBrainz Release Group Id"])
			if mb == nil {
				mb = p.getMultipleMusicBrainzId(tags["MUSICBRAINZ_RELEASEGROUPID"])
			}
		//	audio.SetProviderId(MetadataProvider_MusicBrainzReleaseGroup, mbReleaseGroupId)

			mb = p.getMultipleMusicBrainzId(tags["MusicBrainz Release Track Id"])
			if mb == nil {
				mb = p.getMultipleMusicBrainzId(tags["MUSICBRAINZ_RELEASETRACKID"])
			}
		//	audio.SetProviderId(MetadataProvider_MusicBrainzTrack, mbReleaseTrackId)
	*/
}

func (p *ProbeResultNormalizer) getMultipleMusicBrainzId(value *string) *string {
	var ret string
	if value == nil || strings.TrimSpace(*value) == "" {
		return &ret
	}

	parts := strings.Split(*value, "/")
	for _, part := range parts {
		if trimmedPart := strings.TrimSpace(part); trimmedPart != "" {
			return &trimmedPart
		}
	}

	return &ret
}

func (p *ProbeResultNormalizer) split(val string, allowCommaDelimiter bool) []string {
	// Only use the comma as a delimiter if there are no slashes or pipes.
	// We want to be careful not to split names that have commas in them.
	if !allowCommaDelimiter || containsAny(val, p.nameDelimiters) {
		return splitWithDelimiters(val, p.nameDelimiters)
	} else {
		return strings.FieldsFunc(val, func(r rune) bool {
			return r == ','
		})
	}
}

func splitWithDelimiters(val string, delimiters []rune) []string {
	result := make([]string, 0, 4)
	currentStart := 0
	for i, r := range val {
		for _, d := range delimiters {
			if r == d {
				result = append(result, strings.TrimSpace(val[currentStart:i]))
				currentStart = i + 1
				break
			}
		}
	}
	result = append(result, strings.TrimSpace(val[currentStart:]))
	return result
}

func containsAny(s string, runes []rune) bool {
	for _, r := range runes {
		if strings.ContainsRune(s, r) {
			return true
		}
	}
	return false
}

func (p *ProbeResultNormalizer) splitDistinctArtists(val string, delimiters []rune, splitFeaturing bool) []string {

	if splitFeaturing {
		val = strings.ReplaceAll(strings.ToLower(val), " featuring ", strings.ToLower(p.ArtistReplaceValue))
		val = strings.ReplaceAll(strings.ToLower(val), " feat. ", strings.ToLower(p.ArtistReplaceValue))
	}
	artistsFound := make([]string, 0, len(p.splitWhitelist()))
	for _, whitelist := range p.splitWhitelist() {
		original := val
		val = strings.ReplaceAll(strings.ToLower(val), whitelist, "|")
		if original != val {
			artistsFound = append(artistsFound, strings.TrimSpace(whitelist))
		}
	}

	artists := make([]string, 0, 4)
	for _, artist := range strings.FieldsFunc(val, func(r rune) bool {
		for _, d := range delimiters {
			if r == d {
				return true
			}
		}
		return false
	}) {
		artists = append(artists, strings.TrimSpace(artist))
	}

	return UniqueStrings(append(artists, artists...))
}

func UniqueStrings(input []string) []string {
	unique := make(map[string]struct{}, len(input))
	for _, s := range input {
		unique[s] = struct{}{}
	}

	result := make([]string, 0, len(unique))
	for s := range unique {
		result = append(result, s)
	}
	return result
}

func (p *ProbeResultNormalizer) fetchStudios(info *mediainfo.MediaInfo, tags map[string]string, tagName string) {
	val, ok := tags[tagName]
	if !ok || val == "" {
		return
	}

	studios := p.split(val, true)
	var studioList []string

	for _, studio := range studios {
		if strings.TrimSpace(studio) == "" {
			continue
		}

		// Don't add artist/album artist name to studios, even if it's listed there
		if ContainsIgnoreCase(info.Artists, studio) || ContainsIgnoreCase(info.AlbumArtists, studio) {
			continue
		}

		studioList = append(studioList, studio)
	}

	info.Studios = UniqueIgnoreCase(studioList)
}

func ContainsIgnoreCase(slice []string, target string) bool {
	for _, item := range slice {
		if strings.EqualFold(item, target) {
			return true
		}
	}
	return false
}

func UniqueIgnoreCase(items []string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, item := range items {
		if !seen[strings.ToLower(item)] {
			seen[strings.ToLower(item)] = true
			result = append(result, item)
		}
	}
	return result
}

func (p *ProbeResultNormalizer) fetchGenres(info *mediainfo.MediaInfo, tags map[string]string) {
	genreVal, ok := tags["genre"]
	if !ok || genreVal == "" {
		return
	}

	genres := make([]string, len(info.Genres))
	copy(genres, info.Genres)
	for _, genre := range p.split(genreVal, true) {
		if strings.TrimSpace(genre) != "" {
			genres = append(genres, genre)
		}
	}

	info.Genres = UniqueIgnoreCase(genres)
}

func GetDictionaryTrackOrDiscNumber(tags map[string]string, tagName string) *int {
	disc, ok := tags[tagName]
	if !ok {
		return nil
	}

	i := strings.IndexByte(disc, '/')
	if i >= 0 {
		discNum, err := strconv.Atoi(disc[:i])
		if err == nil {
			return &discNum
		}
	}

	return nil
}

func GetChapterInfo(chapter *MediaChapter) *entities.ChapterInfo {
	info := &entities.ChapterInfo{}

	if chapter.Tags != nil {
		if name, ok := chapter.Tags["title"]; ok {
			info.Name = &name
		}
	}

	// Limit accuracy to milliseconds to match xml saving
	secondsString := chapter.StartTime
	seconds, err := strconv.ParseFloat(secondsString, 64)
	if err == nil {
		ms := math.Round(seconds * 1000)
		info.StartPositionTicks = int64(time.Duration(ms) * time.Millisecond)
	}

	return info
}

func (p *ProbeResultNormalizer) DistinctStrings(strings []string) []string {
	// Implement the DistinctStrings func (p *ProbeResultNormalizer)tion here
	return strings
}

func (p *ProbeResultNormalizer) fetchWtvInfo(video *mediainfo.MediaInfo, data *InternalMediaInfoResult) {
	tags := data.Format.Tags
	if tags == nil {
		return
	}

	if genres, ok := tags["WM/Genre"]; ok && genres != "" {
		genreList := SplitTrimmed(genres, []string{";", "/", ","})
		if len(genreList) > 0 {
			video.Genres = genreList
		}
	}

	if officialRating, ok := tags["WM/ParentalRating"]; ok && officialRating != "" {
		video.OfficialRating = officialRating
	}

	if people, ok := tags["WM/MediaCredits"]; ok && people != "" {
		var personList []dto.BaseItemPerson
		for _, p := range SplitTrimmed(people, []string{";", "/"}) {
			personList = append(personList, dto.BaseItemPerson{Name: p, Type: enums.Actor})
		}
		//		video.People = personList
	}

	if yearStr, ok := tags["WM/OriginalReleaseTime"]; ok {
		if year, err := strconv.Atoi(yearStr); err == nil {
			video.ProductionYear = &year
		}
	}

	if premiereDateStr, ok := tags["WM/MediaOriginalBroadcastDateTime"]; ok {
		if premiereDate, err := time.Parse(time.RFC3339, premiereDateStr); err == nil {
			video.PremiereDate = &premiereDate
		}
	}

	description := GetValueOrDefault(tags, "WM/SubTitleDescription", "")
	subtitle := GetValueOrDefault(tags, "WM/SubTitle", "")

	if subtitle == "" && description != "" && strings.Contains(description, ":") {
		descriptionParts := strings.SplitN(description, ":", 2)
		subtitle := strings.TrimSpace(descriptionParts[0])

		if strings.Contains(subtitle, "/") {
			subtitleParts := strings.SplitN(subtitle, " ", 2)
			numbers := strings.Split(subtitleParts[0], "/")
			if len(numbers) == 2 {
				indexNumber, _ := strconv.Atoi(numbers[0])
				video.IndexNumber = &indexNumber
				// totalEpisodesInSeason, _ = strconv.Atoi(numbers[1])
				description = strings.TrimSpace(strings.Join(subtitleParts[1:], " "))
			}
		} else if strings.Contains(subtitle, ".") {
			subtitleParts := strings.SplitN(subtitle, ".", 2)
			description = strings.TrimSpace(strings.Join(subtitleParts[1:], "."))
		} else {
			description = strings.TrimSpace(subtitle)
		}
	}

	if description != "" {
		video.Overview = description
	}
}

func GetValueOrDefault(m map[string]string, key, defaultValue string) string {
	value, ok := m[key]
	if !ok {
		return defaultValue
	}
	return value
}

func SplitTrimmed(input string, delimiters []string) []string {
	var result []string
	for _, part := range strings.Split(input, delimiters[0]) {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (p *ProbeResultNormalizer) extractTimestamp(video *mediainfo.MediaInfo) {
	if video.VideoType == nil || *video.VideoType != entities.VideoFile {
		return
	}

	if !strings.EqualFold(video.Container, "mpeg2ts") &&
		!strings.EqualFold(video.Container, "m2ts") &&
		!strings.EqualFold(video.Container, "ts") {
		return
	}

	timestamp, err := p.getMpegTimestamp(video.Path)
	if err != nil {
		video.Timestamp = nil
		p.logger.Errorf("Error extracting timestamp info from %s: %v", video.Path, err)
	} else {
		video.Timestamp = timestamp
		p.logger.Debugf("Video has %v timestamp", video.Timestamp)
	}
}

// REVIEW: find out why the byte array needs to be 197 bytes long and comment the reason
func (p *ProbeResultNormalizer) getMpegTimestamp(path string) (*transportstreamtimestamp.TransportStreamTimestamp, error) {
	var timestamp transportstreamtimestamp.TransportStreamTimestamp
	packetBuffer := make([]byte, 197)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	_, err = f.Read(packetBuffer)
	if err != nil {
		return nil, err
	}

	if packetBuffer[0] == 71 {
		timestamp = transportstreamtimestamp.None
		return &timestamp, nil
	}

	if (packetBuffer[4] != 71) || (packetBuffer[196] != 71) {
		timestamp = transportstreamtimestamp.None
		return &timestamp, nil
	}

	if (packetBuffer[0] == 0) && (packetBuffer[1] == 0) && (packetBuffer[2] == 0) && (packetBuffer[3] == 0) {
		timestamp = transportstreamtimestamp.Zero
		return &timestamp, nil
	}

	timestamp = transportstreamtimestamp.Valid
	return &timestamp, nil
}

func PerformerRegex() *regexp.Regexp {
	return regexp.MustCompile(`(?P<name>.*) \((?P<instrument>.*)\)`)
}

package probing

import (
	"encoding/json"
	"fmt"
	"strconv"

	"k8s.io/klog/v2"
)

type MediaStreamInfo struct {
	Index              int                        `json:"index"`
	Profile            string                     `json:"profile"`
	CodecName          string                     `json:"codec_name"`
	CodecLongName      string                     `json:"codec_long_name"`
	CodecType          CodecType                  `json:"codec_type"`
	SampleRate         string                     `json:"sample_rate"`
	Channels           int                        `json:"channels"`
	ChannelLayout      string                     `json:"channel_layout"`
	AverageFrameRate   string                     `json:"avg_frame_rate"`
	Duration           string                     `json:"duration"`
	BitRate            string                     `json:"bit_rate"`
	Width              int                        `json:"width"`
	Refs               int                        `json:"refs"`
	Height             int                        `json:"height"`
	DisplayAspectRatio string                     `json:"display_aspect_ratio"`
	Tags               map[string]string          `json:"tags"`
	BitsPerSample      int                        `json:"bits_per_sample"`
	BitsPerRawSample   int                        `json:"bits_per_raw_sample"`
	RFrameRate         string                     `json:"r_frame_rate"`
	HasBFrames         int                        `json:"has_b_frames"`
	SampleAspectRatio  string                     `json:"sample_aspect_ratio"`
	PixelFormat        string                     `json:"pix_fmt"`
	Level              int                        `json:"level"`
	TimeBase           string                     `json:"time_base"`
	StartTime          string                     `json:"start_time"`
	CodecTimeBase      string                     `json:"codec_time_base"`
	CodecTag           string                     `json:"codec_tag"`
	CodecTagString     string                     `json:"codec_tag_string"`
	SampleFmt          string                     `json:"sample_fmt"`
	DmixMode           string                     `json:"dmix_mode"`
	StartPts           int64                      `json:"start_pts"`
	IsAvc              bool                       `json:"is_avc"`
	NalLengthSize      string                     `json:"nal_length_size"`
	LtrtCmixlev        string                     `json:"ltrt_cmixlev"`
	LtrtSurmixlev      string                     `json:"ltrt_surmixlev"`
	LoroCmixlev        string                     `json:"loro_cmixlev"`
	LoroSurmixlev      string                     `json:"loro_surmixlev"`
	FieldOrder         string                     `json:"field_order"`
	Disposition        map[string]int             `json:"disposition"`
	ColorRange         string                     `json:"color_range"`
	ColorSpace         string                     `json:"color_space"`
	ColorTransfer      string                     `json:"color_transfer"`
	ColorPrimaries     string                     `json:"color_primaries"`
	SideDataList       []*MediaStreamInfoSideData `json:"side_data_list"`
}

func (m *MediaStreamInfo) UnmarshalJSON(data []byte) error {
	type Alias MediaStreamInfo
	aux := &struct {
		*Alias
		IsAvc            string `json:"is_avc"`
		BitsPerRawSample string `json:"bits_per_raw_sample"`
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		klog.Infoln(err)
		return err
	}

	switch aux.IsAvc {
	case "true":
		m.IsAvc = true
	case "false", "":
		m.IsAvc = false
	default:
		return fmt.Errorf("invalid value for is_avc field: %s", aux.IsAvc)
	}

	if aux.BitsPerRawSample == "" {
		return nil
	}

	bitsPerRawSample, err := strconv.Atoi(aux.BitsPerRawSample)
	if err != nil {
		return fmt.Errorf("invalid value BitsPerRawSample field: %s", aux.BitsPerRawSample)
	}

	m.BitsPerRawSample = bitsPerRawSample

	return nil
}

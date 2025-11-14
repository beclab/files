package dlna

import (
// "strings"
)

type CodecProfile struct {
	Type            CodecType
	Conditions      []ProfileCondition
	ApplyConditions []ProfileCondition
	Codec           string
	Container       string
}

func NewCodecProfile() *CodecProfile {
	return &CodecProfile{
		Conditions:      make([]ProfileCondition, 0),
		ApplyConditions: make([]ProfileCondition, 0),
	}
}

/*
func (cp *CodecProfile) GetCodecs() []string {
//    return ContainerProfile.SplitValue(&cp.Codec)
    return SplitValue(&cp.Codec)
}

func (cp *CodecProfile) ContainsContainer(container *string) bool {
//    return ContainerProfile.ContainsContainer(&cp.Container, container)
    return ContainsContainer(&cp.Container, container)
}

func (cp *CodecProfile) ContainsAnyCodec(codec *string, container *string) bool {
    return cp.ContainsAnyCodec(SplitValue(codec), container)
}

func (cp *CodecProfile) ContainsAnyCodec(codecs []string, container *string) bool {
    if !cp.ContainsContainer(container) {
        return false
    }

    codecsInProfile := cp.GetCodecs()
    if len(codecsInProfile) == 0 {
        return true
    }

    for _, val := range codecs {
        for _, profileCodec := range codecsInProfile {
            if strings.EqualFold(val, profileCodec) {
                return true
            }
        }
    }

    return false
}
*/

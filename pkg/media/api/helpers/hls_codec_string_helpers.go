package helpers

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	mp3  = "mp4a.40.34"
	ac3  = "mp4a.a5"
	eac3 = "mp4a.a6"
	flac = "fLaC"
	alac = "alac"
	opus = "Opus"
)

func GetMP3String() string {
	return mp3
}

func GetAACString(profile string) string {
	result := "mp4a"
	if strings.EqualFold(profile, "HE") {
		result += ".40.5"
	} else {
		// Default to LC if profile is invalid
		result += ".40.2"
	}
	return result
}

func GetAC3String() string {
	return ac3
}

func GetEAC3String() string {
	return eac3
}

func GetFLACString() string {
	return flac
}

func GetALACString() string {
	return alac
}

func GetOPUSString() string {
	return opus
}

func GetH264String(profile string, level int) string {
	result := "avc1"
	switch {
	case strings.EqualFold(profile, "high"):
		result += ".6400"
	case strings.EqualFold(profile, "main"):
		result += ".4D40"
	case strings.EqualFold(profile, "baseline"):
		result += ".42E0"
	default:
		// Default to constrained baseline if profile is invalid
		result += ".4240"
	}
	return result + fmt.Sprintf("%02X", level)
}

func GetH265String(profile string, level int) string {
	result := "hvc1"
	if strings.EqualFold(profile, "main10") || strings.EqualFold(profile, "main 10") {
		result += ".2.4"
	} else {
		// Default to main if profile is invalid
		result += ".1.4"
	}
	return result + ".L" + strconv.Itoa(level) + ".B0"
}

func GetAv1String(profile string, level int, tierFlag bool, bitDepth int) string {
	result := "av01"
	switch {
	case strings.EqualFold(profile, "Main"):
		result += ".0"
	case strings.EqualFold(profile, "High"):
		result += ".1"
	case strings.EqualFold(profile, "Professional"):
		result += ".2"
	default:
		// Default to Main
		result += ".0"
	}

	if level <= 0 || level > 31 {
		// Default to the maximum defined level 6.3
		level = 19
	}

	if bitDepth != 8 && bitDepth != 10 && bitDepth != 12 {
		// Default to 8 bits
		bitDepth = 8
	}

	result += "." + fmt.Sprintf("%02d", level)
	if tierFlag {
		result += "H"
	} else {
		result += "M"
	}
	result += "." + fmt.Sprintf("%02d", bitDepth)
	return result
}

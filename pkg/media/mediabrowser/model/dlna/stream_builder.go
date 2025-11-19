package dlna

func NormalizeMediaSourceFormatIntoSingleContainer(inputContainer string, profile *DeviceProfile, dlnaProfileType DlnaProfileType, playProfile *DirectPlayProfile) string {
	if inputContainer == "" {
		return ""
	}

	formats := SplitValue(&inputContainer)

	if profile != nil {
		playProfiles := profile.DirectPlayProfiles
		if playProfile != nil {
			playProfiles = []DirectPlayProfile{*playProfile}
		}
		for _, format := range formats {
			for _, dp := range playProfiles {
				if dp.Type == dlnaProfileType && dp.SupportsContainer(&format) {
					return format
				}
			}
		}
	}

	return formats[0]
}

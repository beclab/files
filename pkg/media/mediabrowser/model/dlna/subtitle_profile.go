package dlna

type SubtitleProfile struct {
	Format    string
	Method    SubtitleDeliveryMethod
	DidlMode  string
	Language  string
	Container string
}

/*
func (sp *SubtitleProfile) GetLanguages() []string {
    return ContainerProfile.SplitValue(&sp.Language)
}

func (sp *SubtitleProfile) SupportsLanguage(subLanguage *string) bool {
    if sp.Language == "" {
        return true
    }

    if subLanguage == nil || *subLanguage == "" {
        subLanguage = stringPtr("und")
    }

    languages := sp.GetLanguages()
    if len(languages) == 0 {
        return true
    }

    for _, lang := range languages {
        if strings.EqualFold(lang, *subLanguage) {
            return true
        }
    }

    return false
}

func stringPtr(s string) *string {
    return &s
}
*/

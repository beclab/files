package globalization

type ILocalizationManager interface {
	// Gets the cultures
	//    GetCultures() []CultureDto

	// Gets the countries
	//    GetCountries() []CountryInfo

	// Gets the parental ratings
	//    GetParentalRatings() []ParentalRating

	// Gets the rating level
	//    GetRatingLevel(rating string, countryCode *string) *int

	// Gets the localized string
	//    GetLocalizedString(phrase, culture string) string
	GetLocalizedString(phrase string) string

	// Gets the localization options
	//    GetLocalizationOptions() []LocalizationOption

	// Finds the language info
	//    FindLanguageInfo(language string) *CultureDto
}

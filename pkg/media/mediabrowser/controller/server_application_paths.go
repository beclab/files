package controller

// ServerApplicationPaths defines methods for accessing server application paths.
// It extends the functionality of ApplicationPaths (assumed base interface).
type IServerApplicationPaths interface {
	// RootFolderPath returns the path to the base root media directory.
	RootFolderPath() string

	// DefaultUserViewsPath returns the path to the default user view directory.
	// Used if no specific user view is defined.
	DefaultUserViewsPath() string

	// PeoplePath returns the path to the People directory.
	PeoplePath() string

	// GenrePath returns the path to the Genre directory.
	GenrePath() string

	// MusicGenrePath returns the path to the Music Genre directory.
	MusicGenrePath() string

	// StudioPath returns the path to the Studio directory.
	StudioPath() string

	// YearPath returns the path to the Year directory.
	YearPath() string

	// UserConfigurationDirectoryPath returns the path to the user configuration directory.
	UserConfigurationDirectoryPath() string

	// DefaultInternalMetadataPath returns the default internal metadata path.
	DefaultInternalMetadataPath() string

	// InternalMetadataPath returns the internal metadata path, either custom or default.
	InternalMetadataPath() string

	// VirtualInternalMetadataPath returns the virtual internal metadata path, either custom or default.
	VirtualInternalMetadataPath() string

	// ArtistsPath returns the path to the Artists directory.
	ArtistsPath() string
}

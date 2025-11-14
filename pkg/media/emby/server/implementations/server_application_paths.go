package implementations

import (
	"files/pkg/media/emby/server/implementations/appbase"
	"path/filepath"
)

// ServerApplicationPaths represents the server application paths
type ServerApplicationPaths struct {
	*appbase.BaseApplicationPaths
	RootFolderPath              string
	DefaultUserViewsPath        string
	DefaultInternalMetadataPath string
	InternalMetadataPath        string
	VirtualInternalMetadataPath string
}

// NewServerApplicationPaths initializes a new instance of ServerApplicationPaths
func NewServerApplicationPaths(
	programDataPath,
	logDirectoryPath,
	configurationDirectoryPath,
	cacheDirectoryPath,
	webDirectoryPath string,
) *ServerApplicationPaths {
	s := &ServerApplicationPaths{
		BaseApplicationPaths: appbase.NewBaseApplicationPaths(
			programDataPath,
			logDirectoryPath,
			configurationDirectoryPath,
			cacheDirectoryPath,
			webDirectoryPath,
		),
	}
	// ProgramDataPath cannot change when the server is running, so cache these to avoid allocations.
	s.RootFolderPath = filepath.Join(s.ProgramDataPath(), "root")
	s.DefaultUserViewsPath = filepath.Join(s.RootFolderPath, "default")
	s.DefaultInternalMetadataPath = filepath.Join(s.ProgramDataPath(), "metadata")
	s.InternalMetadataPath = s.DefaultInternalMetadataPath
	s.VirtualInternalMetadataPath = "%MetadataPath%"
	return s
}

// PeoplePath returns the path to the People directory
func (s *ServerApplicationPaths) PeoplePath() string {
	return filepath.Join(s.InternalMetadataPath, "People")
}

// ArtistsPath returns the path to the artists directory
func (s *ServerApplicationPaths) ArtistsPath() string {
	return filepath.Join(s.InternalMetadataPath, "artists")
}

// GenrePath returns the path to the Genre directory
func (s *ServerApplicationPaths) GenrePath() string {
	return filepath.Join(s.InternalMetadataPath, "Genre")
}

// MusicGenrePath returns the path to the MusicGenre directory
func (s *ServerApplicationPaths) MusicGenrePath() string {
	return filepath.Join(s.InternalMetadataPath, "MusicGenre")
}

// StudioPath returns the path to the Studio directory
func (s *ServerApplicationPaths) StudioPath() string {
	return filepath.Join(s.InternalMetadataPath, "Studio")
}

// YearPath returns the path to the Year directory
func (s *ServerApplicationPaths) YearPath() string {
	return filepath.Join(s.InternalMetadataPath, "Year")
}

// UserConfigurationDirectoryPath returns the path to the user configuration directory
func (s *ServerApplicationPaths) UserConfigurationDirectoryPath() string {
	return filepath.Join(s.ConfigurationDirectoryPath(), "users")
}

// MakeSanityCheckOrThrow performs a sanity check on the paths
func (s *ServerApplicationPaths) MakeSanityCheckOrThrow() error {
	err := s.BaseApplicationPaths.MakeSanityCheckOrThrow()
	if err == nil {
		err = s.CreateAndCheckMarker(s.RootFolderPath, "root", false)
	}
	return err
}

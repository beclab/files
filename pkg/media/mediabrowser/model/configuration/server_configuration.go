package configuration

import (
	"files/pkg/media/mediabrowser/model/drawing"
	"runtime"
)

// MetadataOptions represents configuration options for metadata
type MetadataOptions struct {
	ItemType                 string
	DisabledMetadataFetchers []string
	DisabledImageFetchers    []string
}

// NameValuePair represents a key-value pair
type NameValuePair struct {
	Name  string
	Value string
}

// RepositoryInfo represents plugin repository information
type RepositoryInfo struct {
	// Add relevant fields as needed
}

// PathSubstitution represents path substitution configuration
type PathSubstitution struct {
	// Add relevant fields as needed
}

const (
	ImageResolutionMatchSource drawing.ImageResolution = iota
	// Add other resolution values as needed
)

// CastReceiverApplication represents cast receiver application configuration
type CastReceiverApplication struct {
	// Add relevant fields as needed
}

// TrickplayOptions represents trickplay configuration options
type TrickplayOptions struct {
	// Add relevant fields as needed
}

// ServerConfiguration extends BaseApplicationConfiguration
type ServerConfiguration struct {
	*BaseApplicationConfiguration

	// EnableMetrics indicates whether to enable prometheus metrics exporting
	EnableMetrics bool

	// EnableNormalizedItemByNameIds indicates whether normalized item IDs are enabled
	EnableNormalizedItemByNameIds bool

	// IsPortAuthorized indicates whether the port is authorized
	IsPortAuthorized bool

	// QuickConnectAvailable indicates whether quick connect is available
	QuickConnectAvailable bool

	// EnableCaseSensitiveItemIds indicates whether case-sensitive item IDs are enabled
	EnableCaseSensitiveItemIds bool

	// DisableLiveTvChannelUserDataName indicates whether to disable live TV channel user data name
	DisableLiveTvChannelUserDataName bool

	// MetadataPath is the path for metadata storage
	MetadataPath string

	// PreferredMetadataLanguage is the preferred language for metadata
	PreferredMetadataLanguage string

	// MetadataCountryCode is the country code for metadata
	MetadataCountryCode string

	// SortReplaceCharacters are characters to be replaced with a space in sort names
	SortReplaceCharacters []string

	// SortRemoveCharacters are characters to be removed from sort names
	SortRemoveCharacters []string

	// SortRemoveWords are words to be removed from sort names
	SortRemoveWords []string

	// MinResumePct is the minimum percentage of an item that must be played for playstate updates
	MinResumePct int

	// MaxResumePct is the maximum percentage of an item that can be played while saving playstate
	MaxResumePct int

	// MinResumeDurationSeconds is the minimum duration for playstate updates
	MinResumeDurationSeconds int

	// MinAudiobookResume is the minimum minutes of an audiobook for playstate updates
	MinAudiobookResume int

	// MaxAudiobookResume is the maximum minutes of an audiobook for saving playstate
	MaxAudiobookResume int

	// InactiveSessionThreshold is the threshold in minutes for closing inactive sessions
	InactiveSessionThreshold int

	// LibraryMonitorDelay is the delay in seconds after a file system change
	LibraryMonitorDelay int

	// LibraryUpdateDuration is the duration in seconds after a library update
	LibraryUpdateDuration int

	// CacheSize is the maximum amount of items to cache
	CacheSize int

	// ImageSavingConvention is the convention for saving images
	ImageSavingConvention ImageSavingConvention

	// MetadataOptions are the metadata configuration options
	MetadataOptions []MetadataOptions

	// SkipDeserializationForBasicTypes indicates whether to skip deserialization for basic types
	SkipDeserializationForBasicTypes bool

	// ServerName is the name of the server
	ServerName string

	// UICulture is the UI culture setting
	UICulture string

	// SaveMetadataHidden indicates whether to save metadata as hidden
	SaveMetadataHidden bool

	// ContentTypes are the content type configurations
	ContentTypes []NameValuePair

	// RemoteClientBitrateLimit is the bitrate limit for remote clients
	RemoteClientBitrateLimit int

	// EnableFolderView indicates whether folder view is enabled
	EnableFolderView bool

	// EnableGroupingMoviesIntoCollections indicates whether to group movies into collections
	EnableGroupingMoviesIntoCollections bool

	// EnableGroupingShowsIntoCollections indicates whether to group shows into collections
	EnableGroupingShowsIntoCollections bool

	// DisplaySpecialsWithinSeasons indicates whether to display specials within seasons
	DisplaySpecialsWithinSeasons bool

	// CodecsUsed are the codecs used by the server
	CodecsUsed []string

	// PluginRepositories are the plugin repository configurations
	PluginRepositories []RepositoryInfo

	// EnableExternalContentInSuggestions indicates whether to enable external content in suggestions
	EnableExternalContentInSuggestions bool

	// ImageExtractionTimeoutMs is the timeout for image extraction in milliseconds
	ImageExtractionTimeoutMs int

	// PathSubstitutions are the path substitution configurations
	PathSubstitutions []PathSubstitution

	// EnableSlowResponseWarning indicates whether to log slow server responses as warnings
	EnableSlowResponseWarning bool

	// SlowResponseThresholdMs is the threshold for slow response warnings in milliseconds
	SlowResponseThresholdMs int64

	// CorsHosts are the allowed CORS hosts
	CorsHosts []string

	// ActivityLogRetentionDays is the number of days to retain activity logs
	ActivityLogRetentionDays *int

	// LibraryScanFanoutConcurrency is the concurrency level for library scans
	LibraryScanFanoutConcurrency int

	// LibraryMetadataRefreshConcurrency is the concurrency level for metadata refreshes
	LibraryMetadataRefreshConcurrency int

	// AllowClientLogUpload indicates whether clients can upload logs
	AllowClientLogUpload bool

	// DummyChapterDuration is the duration for dummy chapters in seconds
	DummyChapterDuration int

	// ChapterImageResolution is the resolution for chapter images
	ChapterImageResolution drawing.ImageResolution

	// ParallelImageEncodingLimit is the limit for parallel image encoding
	ParallelImageEncodingLimit int

	// CastReceiverApplications are the cast receiver application configurations
	CastReceiverApplications []CastReceiverApplication

	// TrickplayOptions are the trickplay configuration options
	TrickplayOptions TrickplayOptions

	// EnableLegacyAuthorization indicates whether old authorization methods are allowed
	EnableLegacyAuthorization bool
}

// NewServerConfiguration initializes a new ServerConfiguration
func NewServerConfiguration() *ServerConfiguration {
	activityLogRetentionDays := 30
	return &ServerConfiguration{
		BaseApplicationConfiguration:     NewBaseApplicationConfiguration(),
		EnableMetrics:                    false,
		EnableNormalizedItemByNameIds:    true,
		QuickConnectAvailable:            true,
		EnableCaseSensitiveItemIds:       true,
		DisableLiveTvChannelUserDataName: true,
		MetadataPath:                     "",
		PreferredMetadataLanguage:        "en",
		MetadataCountryCode:              "US",
		SortReplaceCharacters:            []string{".", "+", "%"},
		SortRemoveCharacters:             []string{",", "&", "-", "{", "}", "'"},
		SortRemoveWords:                  []string{"the", "a", "an"},
		MinResumePct:                     5,
		MaxResumePct:                     90,
		MinResumeDurationSeconds:         300,
		MinAudiobookResume:               5,
		MaxAudiobookResume:               5,
		LibraryMonitorDelay:              60,
		LibraryUpdateDuration:            30,
		CacheSize:                        runtime.NumCPU() * 100,
		MetadataOptions: []MetadataOptions{
			{ItemType: "Book"},
			{ItemType: "Movie"},
			{
				ItemType:                 "MusicVideo",
				DisabledMetadataFetchers: []string{"The Open Movie Database"},
				DisabledImageFetchers:    []string{"The Open Movie Database"},
			},
			{ItemType: "Series"},
			{
				ItemType:                 "MusicAlbum",
				DisabledMetadataFetchers: []string{"TheAudioDB"},
			},
			{
				ItemType:                 "MusicArtist",
				DisabledMetadataFetchers: []string{"TheAudioDB"},
			},
			{ItemType: "BoxSet"},
			{ItemType: "Season"},
			{ItemType: "Episode"},
		},
		SkipDeserializationForBasicTypes:   true,
		ServerName:                         "",
		UICulture:                          "en-US",
		SaveMetadataHidden:                 false,
		ContentTypes:                       []NameValuePair{},
		CodecsUsed:                         []string{},
		PluginRepositories:                 []RepositoryInfo{},
		EnableExternalContentInSuggestions: true,
		PathSubstitutions:                  []PathSubstitution{},
		EnableSlowResponseWarning:          true,
		SlowResponseThresholdMs:            500,
		CorsHosts:                          []string{"*"},
		ActivityLogRetentionDays:           &activityLogRetentionDays,
		AllowClientLogUpload:               true,
		ChapterImageResolution:             ImageResolutionMatchSource,
		CastReceiverApplications:           []CastReceiverApplication{},
		TrickplayOptions:                   TrickplayOptions{},
		EnableLegacyAuthorization:          true,
	}
}

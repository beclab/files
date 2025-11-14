package enums

type PersonKind int

const (
	Unknown PersonKind = iota
	Actor
	Director
	Composer
	Writer
	GuestStar
	Producer
	Conductor
	Lyricist
	Arranger
	Engineer
	Mixer
	Remixer
	Creator
	Artist
	AlbumArtist
	Author
	Illustrator
	Penciller
	Inker
	Colorist
	Letterer
	CoverArtist
	Editor
	Translator
)

func (p PersonKind) String() string {
	switch p {
	case Unknown:
		return "Unknown"
	case Actor:
		return "Actor"
	case Director:
		return "Director"
	case Composer:
		return "Composer"
	case Writer:
		return "Writer"
	case GuestStar:
		return "GuestStar"
	case Producer:
		return "Producer"
	case Conductor:
		return "Conductor"
	case Lyricist:
		return "Lyricist"
	case Arranger:
		return "Arranger"
	case Engineer:
		return "Engineer"
	case Mixer:
		return "Mixer"
	case Remixer:
		return "Remixer"
	case Creator:
		return "Creator"
	case Artist:
		return "Artist"
	case AlbumArtist:
		return "AlbumArtist"
	case Author:
		return "Author"
	case Illustrator:
		return "Illustrator"
	case Penciller:
		return "Penciller"
	case Inker:
		return "Inker"
	case Colorist:
		return "Colorist"
	case Letterer:
		return "Letterer"
	case CoverArtist:
		return "CoverArtist"
	case Editor:
		return "Editor"
	case Translator:
		return "Translator"
	default:
		return "Unknown"
	}
}

package entities

type ImageType int

const (
	Primary ImageType = iota
	Art
	Backdrop
	Banner
	Logo
	Thumb
	Disc
	Box
	// Screenshot is obsolete and not included
	Menu
	Chapter
	BoxRear
	Profile
)

func (i ImageType) String() string {
	switch i {
	case Primary:
		return "Primary"
	case Art:
		return "Art"
	case Backdrop:
		return "Backdrop"
	case Banner:
		return "Banner"
	case Logo:
		return "Logo"
	case Thumb:
		return "Thumb"
	case Disc:
		return "Disc"
	case Box:
		return "Box"
	case Menu:
		return "Menu"
	case Chapter:
		return "Chapter"
	case BoxRear:
		return "BoxRear"
	case Profile:
		return "Profile"
	default:
		return "Unknown"
	}
}

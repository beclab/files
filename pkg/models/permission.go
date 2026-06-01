package models

// Level is the unified permission level returned by every storage
// backend's permission check for a single resource. Higher values include
// lower ones.
//
// A Level answers "what can this user do on this resource", and is distinct
// from the platform identity role in the integration package
// (owner/admin/normal, which answers "who is this user on the instance").
// In particular LevelAdmin means full control over a resource and is NOT
// the platform admin role; the two only meet where a platform role is used
// to derive a Level (see drive/Common in the posix driver).
type Level int

const (
	LevelNone Level = iota
	LevelRead
	LevelWrite
	LevelAdmin
)

// Action is the operation a caller wants to perform on a resource.
type Action int

const (
	ActionList Action = iota
	ActionRead
	ActionPreview
	ActionDownload
	ActionWrite
	ActionUpload
	ActionDelete
	ActionShareManage
)

// Allow reports whether this level permits the given action.
func (l Level) Allow(a Action) bool {
	switch a {
	case ActionList, ActionRead, ActionPreview, ActionDownload:
		return l >= LevelRead
	case ActionWrite, ActionUpload, ActionDelete:
		return l >= LevelWrite
	case ActionShareManage:
		return l >= LevelAdmin
	default:
		return false
	}
}

func (l Level) String() string {
	switch l {
	case LevelRead:
		return "read"
	case LevelWrite:
		return "write"
	case LevelAdmin:
		return "admin"
	default:
		return "none"
	}
}

// LevelFromSyncPermission maps a Seafile permission string to a Level.
// Empty means no access; "r"/"preview"/"cloud-edit" are read-only;
// "rw" is read-write; "admin" is the highest.
func LevelFromSyncPermission(perm string) Level {
	switch perm {
	case "r", "preview", "cloud-edit":
		return LevelRead
	case "rw":
		return LevelWrite
	case "admin":
		return LevelAdmin
	default:
		return LevelNone
	}
}

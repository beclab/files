package integration

import "strings"

// Platform roles describe a user's identity on the Olares instance as a
// whole (owner/admin/normal), sourced from the bytetrade.io/owner-role
// annotation. They are global and NOT tied to any file or directory.
//
// This is distinct from the per-resource permission in models.Level
// (None/Read/Write/Admin): a platform role answers "who is this user on
// the instance", while a Level answers "what can this user do on this
// resource". The only place the two meet is drive/Common, where the
// platform role is used as an input to derive a resource Level.
const (
	PlatformRoleOwner  = "owner"
	PlatformRoleAdmin  = "admin"
	PlatformRoleNormal = "normal"
)

// GetPlatformRole returns the platform identity role of the named user and
// whether the user was found. The role comes from the
// bytetrade.io/owner-role annotation surfaced by GetUsers.
func (i *integration) GetPlatformRole(name string) (string, bool) {
	if name == "" {
		return "", false
	}
	for _, u := range i.GetUsers() {
		if u.Name == name {
			return u.Role, true
		}
	}
	return "", false
}

// UserExists reports whether the named user is known to the platform.
func (i *integration) UserExists(name string) bool {
	_, ok := i.GetPlatformRole(name)
	return ok
}

// IsPlatformAdmin reports whether the named user has owner- or admin-level
// platform role. This is an OS-identity check, NOT a resource permission;
// it must not be confused with models.LevelAdmin.
func (i *integration) IsPlatformAdmin(name string) bool {
	role, ok := i.GetPlatformRole(name)
	if !ok {
		return false
	}
	switch strings.ToLower(role) {
	case PlatformRoleOwner, PlatformRoleAdmin:
		return true
	default:
		return false
	}
}

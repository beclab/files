package common

import (
	"regexp"
	"strings"
)

// smbUserNameRegex aligns with Linux NAME_REGEX accepted by useradd:
// first char is lowercase letter or underscore, rest are [a-z0-9_-],
// total length 6-16, must not end with a hyphen.
var smbUserNameRegex = regexp.MustCompile(`^[a-z_][a-z0-9_-]{5,15}$`)

// smbUserNameBlacklist holds names that match the regex and length
// constraints but collide with system accounts inside the samba container.
var smbUserNameBlacklist = map[string]struct{}{
	"daemon":   {},
	"shutdown": {},
	"nobody":   {},
}

// ValidateSmbUserName validates the requested SMB user name against the
// length, character-set and reserved-name rules, and ensures it does not
// collide with the current owner.
func ValidateSmbUserName(userName, owner string) string {
	if l := len(userName); l < 6 || l > 16 {
		return ErrorMessageSmbUserNameLength
	}
	if !smbUserNameRegex.MatchString(userName) {
		return ErrorMessageSmbUserNameInvalid
	}
	if strings.HasSuffix(userName, "-") {
		return ErrorMessageSmbUserNameInvalid
	}
	if _, ok := smbUserNameBlacklist[userName]; ok {
		return ErrorMessageSmbUserNameReserved
	}
	if userName == owner {
		return ErrorMessageSmbUserNameSameAsOwner
	}
	return ""
}

// ValidateSmbPassword enforces length 6-16, printable ASCII only,
// and rejects leading or trailing whitespace.
func ValidateSmbPassword(password string) string {
	if l := len(password); l < 6 || l > 16 {
		return ErrorMessageSmbPasswordLength
	}
	if strings.HasPrefix(password, " ") || strings.HasSuffix(password, " ") {
		return ErrorMessageSmbPasswordInvalid
	}
	for i := 0; i < len(password); i++ {
		b := password[i]
		if b < 0x20 || b > 0x7E {
			return ErrorMessageSmbPasswordInvalid
		}
	}
	return ""
}

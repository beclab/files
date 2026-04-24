package common

import (
	"errors"
	"net/http"
	"os"
)

var (
	ErrEmptyKey             = errors.New("empty key")
	ErrExist                = errors.New("the resource already exists")
	ErrNotExist             = errors.New("the resource does not exist")
	ErrEmptyPassword        = errors.New("password is empty")
	ErrEmptyUsername        = errors.New("username is empty")
	ErrEmptyRequest         = errors.New("empty request")
	ErrScopeIsRelative      = errors.New("scope is a relative path")
	ErrInvalidDataType      = errors.New("invalid data type")
	ErrIsDirectory          = errors.New("file is directory")
	ErrInvalidOption        = errors.New("invalid option")
	ErrInvalidAuthMethod    = errors.New("invalid auth method")
	ErrPermissionDenied     = errors.New("permission denied")
	ErrInvalidRequestParams = errors.New("invalid request params")
	ErrSourceIsParent       = errors.New("source is parent")
	ErrRootUserDeletion     = errors.New("user with id 1 can't be deleted")
)

// SanitizeFsError returns an error whose message no longer contains the
// possibly very long source/destination paths that os.Rename / os.Open and
// other filesystem syscalls embed in *os.LinkError, *os.PathError and
// *os.SyscallError. The underlying syscall message (e.g. "file name too long")
// is preserved, so callers can still understand what went wrong.
//
// If err is nil or none of the wrappers above apply, the original error is
// returned unchanged.
func SanitizeFsError(err error) error {
	if err == nil {
		return nil
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) && linkErr.Err != nil {
		return errors.New(linkErr.Err.Error())
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) && pathErr.Err != nil {
		return errors.New(pathErr.Err.Error())
	}
	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) && syscallErr.Err != nil {
		return errors.New(syscallErr.Err.Error())
	}
	return err
}

func ErrToStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case os.IsPermission(err):
		return http.StatusForbidden
	case os.IsNotExist(err), err == ErrNotExist:
		return http.StatusNotFound
	case os.IsExist(err), err == ErrExist:
		return http.StatusConflict
	case errors.Is(err, ErrPermissionDenied):
		return http.StatusForbidden
	case errors.Is(err, ErrInvalidRequestParams):
		return http.StatusBadRequest
	case errors.Is(err, ErrRootUserDeletion):
		return http.StatusForbidden
	case err.Error() == "file size exceeds 4GB":
		return http.StatusRequestEntityTooLarge
	default:
		return http.StatusInternalServerError
	}
}

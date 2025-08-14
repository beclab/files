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

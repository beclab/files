package archive

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"files/pkg/archive/reader"
	"files/pkg/archive/sevenz"
)

func TestClassifyStreamError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{sevenz.ErrPasswordInvalid, "password_invalid"},
		{sevenz.ErrPasswordRequired, "password_required"},
		{sevenz.ErrCorrupt, "archive_corrupt"},
		{sevenz.ErrVolumeMissing, "volume_missing"},
		{context.Canceled, "canceled"},
		{errors.New("foo: no such file or directory"), "not_found"},
		{errors.New("unrelated"), "internal"},
	}
	for _, c := range cases {
		if got := classifyStreamError(c.err); got != c.want {
			t.Errorf("classifyStreamError(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestClassifyEntryError(t *testing.T) {
	cases := []struct {
		err      error
		wantCode string
		wantHTTP int
	}{
		{reader.ErrEncryptedEntry, "password_required", http.StatusUnauthorized},
		{reader.ErrEntryTooLarge, "entry_too_large", http.StatusRequestEntityTooLarge},
		{sevenz.ErrPasswordInvalid, "password_invalid", http.StatusUnauthorized},
		{sevenz.ErrPasswordRequired, "password_required", http.StatusUnauthorized},
		{sevenz.ErrCorrupt, "archive_corrupt", http.StatusBadRequest},
		{sevenz.ErrVolumeMissing, "volume_missing", http.StatusBadRequest},
		{errors.New("entry not found: foo"), "not_found", http.StatusNotFound},
		{errors.New("boom"), "internal", http.StatusInternalServerError},
	}
	for _, c := range cases {
		gotCode, gotHTTP := classifyEntryError(c.err)
		if gotCode != c.wantCode || gotHTTP != c.wantHTTP {
			t.Errorf("classifyEntryError(%v) = (%q,%d), want (%q,%d)",
				c.err, gotCode, gotHTTP, c.wantCode, c.wantHTTP)
		}
	}
}

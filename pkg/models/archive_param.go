package models

import (
	"errors"
	"files/pkg/common"
	"strings"
)

// ArchiveOption carries the format-level parameters for one compress /
// extract job. Password must never end up in an access log; the handler
// layer reads it from the X-Archive-Password header rather than the
// request body.
type ArchiveOption struct {
	Format           string `json:"format"`
	Level            int    `json:"level"`
	Password         string `json:"-"`
	VolumeSizeMB     int64  `json:"volumeSizeMB"`
	PreserveSymlinks bool   `json:"preserveSymlinks"`
	Conflict         string `json:"conflict"`
	// HeaderEncrypt drives 7z's -mhe=on (encrypt the central
	// directory). Only meaningful for the 7z format; auto-enabled by
	// NormalizeForCompress when Format == "7z" && Password != "".
	HeaderEncrypt bool `json:"-"`
}

// NormalizeForCompress fills in defaults and validates the option set
// for a compress request. dstName is used to infer Format from its
// suffix when Format is empty.
func (o *ArchiveOption) NormalizeForCompress(dstName string) error {
	if o == nil {
		return errors.New("archive option is nil")
	}
	if o.Format == "" {
		o.Format = common.ArchiveFormatFromName(dstName)
	}
	if o.Format == "" {
		return errors.New("cannot infer archive format from destination name")
	}
	if !common.ListContains(common.ArchiveFormatsWrite, o.Format) {
		return errors.New("unsupported archive format: " + o.Format)
	}
	if o.Conflict == "" {
		o.Conflict = common.ArchiveConflictRename
	}
	if o.Level < 0 || o.Level > 9 {
		return errors.New("compression level must be in [0,9]")
	}
	if o.Level == 0 {
		o.Level = 5
	}
	if o.Password != "" && !common.ListContains(common.ArchiveFormatsWithPassword, o.Format) {
		return errors.New("password only supported for zip or 7z")
	}
	if o.VolumeSizeMB < 0 {
		return errors.New("volumeSizeMB must be >= 0")
	}
	if o.VolumeSizeMB > 0 && !common.ListContains(common.ArchiveFormatsWithVolume, o.Format) {
		return errors.New("multi-volume only supported for zip or 7z")
	}
	if o.Format == common.ArchiveFormat7z && o.Password != "" {
		o.HeaderEncrypt = true
	}
	return nil
}

// NormalizeForExtract fills in defaults and validates the option set
// for an extract request. srcName is used to infer Format from its
// suffix.
func (o *ArchiveOption) NormalizeForExtract(srcName string) error {
	if o == nil {
		return errors.New("archive option is nil")
	}
	if o.Format == "" {
		// Multi-volume archives end with .001; strip it before
		// inferring the underlying format.
		base := strings.TrimSuffix(srcName, ".001")
		o.Format = common.ArchiveFormatFromName(base)
	}
	if o.Format == "" {
		return errors.New("cannot infer archive format from source name")
	}
	if !common.ListContains(common.ArchiveFormatsRead, o.Format) {
		return errors.New("unsupported archive format: " + o.Format)
	}
	if o.Conflict == "" {
		o.Conflict = common.ArchiveConflictRename
	}
	return nil
}

package models

import (
	"errors"
	"files/pkg/common"
	"strings"
)

// ArchiveOption 描述一次压缩/解压所需的格式参数。
// Password 字段不应该出现在任何 access log；handler 层从 header 接入。
type ArchiveOption struct {
	Format           string `json:"format"`
	Level            int    `json:"level"`
	Password         string `json:"-"`
	VolumeSizeMB     int64  `json:"volumeSizeMB"`
	PreserveSymlinks bool   `json:"preserveSymlinks"`
	Conflict         string `json:"conflict"`
	// HeaderEncrypt 控制 7z -mhe=on；仅 7z 格式有效，加密时默认 true。
	HeaderEncrypt bool `json:"-"`
}

// NormalizeForCompress 将 ArchiveOption 在压缩场景下补齐默认值并校验。
// dstName 用于按后缀推断 Format（当 Format 为空时）。
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

// NormalizeForExtract 将 ArchiveOption 在解压场景下补齐默认值并校验。
// srcName 用于按后缀推断 Format。
func (o *ArchiveOption) NormalizeForExtract(srcName string) error {
	if o == nil {
		return errors.New("archive option is nil")
	}
	if o.Format == "" {
		// 多卷归档以 .001 结尾时去掉后缀再推断。
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

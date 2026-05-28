package models

import (
	"files/pkg/common"
	"testing"
)

func TestArchiveOptionNormalizeForCompress(t *testing.T) {
	tests := []struct {
		name     string
		opt      ArchiveOption
		dst      string
		wantErr  bool
		wantFmt  string
		wantConf string
		wantHE   bool
	}{
		{
			name:     "infer from .zip suffix",
			opt:      ArchiveOption{},
			dst:      "out.zip",
			wantFmt:  common.ArchiveFormatZip,
			wantConf: common.ArchiveConflictRename,
		},
		{
			name:     "explicit 7z + password sets HE",
			opt:      ArchiveOption{Format: common.ArchiveFormat7z, Password: "x"},
			dst:      "x.7z",
			wantFmt:  common.ArchiveFormat7z,
			wantConf: common.ArchiveConflictRename,
			wantHE:   true,
		},
		{
			name:    "tar.gz inferred",
			opt:     ArchiveOption{},
			dst:     "out.tar.gz",
			wantFmt: common.ArchiveFormatTarGz,
		},
		{
			name:    "unsupported format errors",
			opt:     ArchiveOption{Format: "rar"},
			dst:     "x.rar",
			wantErr: true,
		},
		{
			name:    "password on tar errors",
			opt:     ArchiveOption{Format: common.ArchiveFormatTar, Password: "p"},
			dst:     "x.tar",
			wantErr: true,
		},
		{
			name:    "volume on tar errors",
			opt:     ArchiveOption{Format: common.ArchiveFormatTar, VolumeSizeMB: 100},
			dst:     "x.tar",
			wantErr: true,
		},
		{
			name:    "level out of range",
			opt:     ArchiveOption{Format: common.ArchiveFormatZip, Level: 11},
			dst:     "x.zip",
			wantErr: true,
		},
		{
			name:    "no format and no suffix",
			opt:     ArchiveOption{},
			dst:     "noext",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := tc.opt
			err := o.NormalizeForCompress(tc.dst)
			if (err != nil) != tc.wantErr {
				t.Fatalf("got err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if tc.wantFmt != "" && o.Format != tc.wantFmt {
				t.Errorf("format=%q want %q", o.Format, tc.wantFmt)
			}
			if tc.wantConf != "" && o.Conflict != tc.wantConf {
				t.Errorf("conflict=%q want %q", o.Conflict, tc.wantConf)
			}
			if o.HeaderEncrypt != tc.wantHE {
				t.Errorf("HE=%v want %v", o.HeaderEncrypt, tc.wantHE)
			}
		})
	}
}

func TestArchiveOptionNormalizeForExtract(t *testing.T) {
	o := &ArchiveOption{}
	if err := o.NormalizeForExtract("foo.zip"); err != nil {
		t.Errorf("zip: %v", err)
	} else if o.Format != common.ArchiveFormatZip || o.Conflict != common.ArchiveConflictRename {
		t.Errorf("zip wrong: %+v", o)
	}

	o = &ArchiveOption{}
	if err := o.NormalizeForExtract("foo.7z.001"); err != nil {
		t.Errorf("7z.001: %v", err)
	} else if o.Format != common.ArchiveFormat7z {
		t.Errorf("7z.001 format: %+v", o)
	}

	o = &ArchiveOption{}
	if err := o.NormalizeForExtract("foo.unknown"); err == nil {
		t.Errorf("expected error for unknown ext")
	}

	if err := (*ArchiveOption)(nil).NormalizeForExtract("foo.zip"); err == nil {
		t.Errorf("nil receiver should error")
	}
}

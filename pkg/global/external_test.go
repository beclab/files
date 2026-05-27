package global

import (
	"files/pkg/files"
	"testing"
)

func strPtr(v string) *string       { return &v }
func int64Ptr(v int64) *int64       { return &v }
func float64Ptr(v float64) *float64 { return &v }
func boolPtr(v bool) *bool          { return &v }

func TestUpdateReportedMountedPreservesMissingFieldsFromPolledBase(t *testing.T) {
	baseReadOnly := false
	m := &Mount{
		Mounted: make(map[string]*files.DiskInfo),
		polledMounted: map[string]*files.DiskInfo{
			"disk-1": {
				Path:          "disk-1",
				Type:          "usb",
				Fstype:        "ext4",
				Total:         1000,
				Free:          800,
				ReadOnly:      &baseReadOnly,
				IDSerial:      "serial-a",
				PartitionUUID: "partition-a",
			},
		},
		reportedMounted: make(map[string]*files.DiskInfo),
	}

	m.UpdateReportedMounted([]*MountedPatch{
		{
			Path:        strPtr("disk-1"),
			Used:        int64Ptr(200),
			UsedPercent: float64Ptr(20),
		},
	})

	got, ok := m.Mounted["disk-1"]
	if !ok {
		t.Fatalf("merged mount does not contain disk-1")
	}
	if got.Fstype != "ext4" {
		t.Fatalf("missing-field preservation failed: got fstype=%q", got.Fstype)
	}
	if got.Total != 1000 || got.Free != 800 {
		t.Fatalf("capacity fields should remain from polled base, got total=%d free=%d", got.Total, got.Free)
	}
	if got.Used != 200 || got.UsedPercent != 20 {
		t.Fatalf("reported fields should override base, got used=%d usedPercent=%f", got.Used, got.UsedPercent)
	}
	if got.ReadOnly == nil || *got.ReadOnly != false {
		t.Fatalf("base read_only should be preserved")
	}
	if got.IDSerial != "serial-a" || got.PartitionUUID != "partition-a" {
		t.Fatalf("base identity fields should be preserved")
	}
}

func TestUpdateReportedMountedAppliesExplicitZeroAndFalse(t *testing.T) {
	baseReadOnly := true
	m := &Mount{
		Mounted: make(map[string]*files.DiskInfo),
		polledMounted: map[string]*files.DiskInfo{
			"disk-2": {
				Path:     "disk-2",
				Used:     100,
				Invalid:  true,
				ReadOnly: &baseReadOnly,
			},
		},
		reportedMounted: make(map[string]*files.DiskInfo),
	}

	m.UpdateReportedMounted([]*MountedPatch{
		{
			Path:     strPtr("disk-2"),
			Used:     int64Ptr(0),
			Invalid:  boolPtr(false),
			ReadOnly: boolPtr(false),
		},
	})

	got, ok := m.Mounted["disk-2"]
	if !ok {
		t.Fatalf("merged mount does not contain disk-2")
	}
	if got.Used != 0 {
		t.Fatalf("explicit zero should be applied, got used=%d", got.Used)
	}
	if got.Invalid {
		t.Fatalf("explicit false should be applied, got invalid=true")
	}
	if got.ReadOnly == nil || *got.ReadOnly {
		t.Fatalf("explicit read_only=false should be applied")
	}
}

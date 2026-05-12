package tasks

import (
	"reflect"
	"testing"
)

func TestNormalizeSyncDirentList(t *testing.T) {
	raw := []interface{}{
		map[string]interface{}{"name": "a.txt", "type": "file"},
		map[string]interface{}{"name": "dir", "type": "dir"},
	}

	got, err := normalizeSyncDirentList(raw, "/")
	if err != nil {
		t.Fatalf("normalizeSyncDirentList returned error: %v", err)
	}

	want := []map[string]interface{}{
		{"name": "a.txt", "type": "file"},
		{"name": "dir", "type": "dir"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeSyncDirentList = %#v, want %#v", got, want)
	}
}

func TestNormalizeSyncDirentListRejectsInvalidList(t *testing.T) {
	_, err := normalizeSyncDirentList(map[string]interface{}{}, "/bad")
	if err == nil {
		t.Fatal("normalizeSyncDirentList returned nil error for invalid list")
	}
	if err.Error() != "invalid directory format" {
		t.Fatalf("normalizeSyncDirentList error = %q, want %q", err.Error(), "invalid directory format")
	}
}

func TestNormalizeSyncDirentListRejectsInvalidItem(t *testing.T) {
	_, err := normalizeSyncDirentList([]interface{}{"not-a-dirent"}, "/bad")
	if err == nil {
		t.Fatal("normalizeSyncDirentList returned nil error for invalid item")
	}
	if err.Error() != "invalid directory item type" {
		t.Fatalf("normalizeSyncDirentList error = %q, want %q", err.Error(), "invalid directory item type")
	}
}

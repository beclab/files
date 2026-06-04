package sevenz

import (
	"errors"
	"testing"
)

// Parser-only unit tests; they don't need 7z installed and always run.

func TestParsePercent(t *testing.T) {
	cases := []struct {
		in      string
		wantN   int
		wantOk  bool
	}{
		{"  3% 4 + foo/bar.txt", 3, true},
		{" 99% 12 + done.bin", 99, true},
		{"100% Files: 100", 100, true},
		{"Scanning the drive: 12 files", 0, false},
		{"Creating archive: foo.zip", 0, false},
		{"", 0, false},
		{"abc% nope", 0, false},
		{"200% out of range", 0, false},
	}
	for _, c := range cases {
		gotN, gotOk := parsePercent(c.in)
		if gotN != c.wantN || gotOk != c.wantOk {
			t.Errorf("parsePercent(%q) = (%d, %v), want (%d, %v)", c.in, gotN, gotOk, c.wantN, c.wantOk)
		}
	}
}

func TestSplitKV(t *testing.T) {
	if k, v, ok := splitKV("Path = foo/bar.txt"); !ok || k != "Path" || v != "foo/bar.txt" {
		t.Errorf("splitKV simple failed: %q %q %v", k, v, ok)
	}
	if k, v, ok := splitKV("Size = 12345"); !ok || k != "Size" || v != "12345" {
		t.Errorf("splitKV num failed: %q %q %v", k, v, ok)
	}
	if _, _, ok := splitKV("no equals here"); ok {
		t.Errorf("splitKV should fail for no-= line")
	}
	if k, v, ok := splitKV("Path = with = embedded"); !ok || k != "Path" || v != "with = embedded" {
		t.Errorf("splitKV embedded-= failed: %q %q %v", k, v, ok)
	}
}

func TestParseEntry(t *testing.T) {
	m := map[string]string{
		"Path":     "dir/file.txt",
		"Size":     "100",
		"Modified": "2024-05-01 12:30:00",
	}
	e, ok := parseEntry(m, "")
	if !ok || e.Path != "dir/file.txt" || e.Size != 100 || e.IsDir {
		t.Errorf("parseEntry file: %+v ok=%v", e, ok)
	}

	mDir := map[string]string{"Path": "dir", "Folder": "+", "Modified": "2024-05-01 12:30:00"}
	eD, ok := parseEntry(mDir, "")
	if !ok || !eD.IsDir {
		t.Errorf("parseEntry dir: %+v ok=%v", eD, ok)
	}

	mEnc := map[string]string{"Path": "secret.txt", "Encrypted": "+"}
	eE, _ := parseEntry(mEnc, "")
	if !eE.Encrypted {
		t.Errorf("parseEntry encrypted flag missed")
	}

	if _, ok := parseEntry(map[string]string{}, ""); ok {
		t.Errorf("empty map should not produce entry")
	}

	mNoPath := map[string]string{"Size": "1234", "Method": "LZMA2:25"}
	eF, ok := parseEntry(mNoPath, "3X")
	if !ok || eF.Path != "3X" || eF.Size != 1234 {
		t.Errorf("parseEntry single-stream fallback: %+v ok=%v", eF, ok)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		out  string
		want error
	}{
		{"... ERROR: Wrong password\n", ErrPasswordInvalid},
		{"... Cannot open encrypted archive. Wrong password?\n", ErrPasswordInvalid},
		{"Enter password (will not be echoed):\n", ErrPasswordRequired},
		{"Can not find the file for archive volume: foo.7z.002\n", ErrVolumeMissing},
		{"foo.zip is not archive\n", ErrCorrupt},
		{"Headers Error in foo.7z\n", ErrCorrupt},
		{"Unexpected end of archive\n", ErrCorrupt},
		{"some unrelated failure", errors.New("orig")},
	}
	for _, c := range cases {
		got := Classify(errors.New("orig"), c.out)
		if !errors.Is(got, c.want) && got.Error() != c.want.Error() {
			t.Errorf("Classify(%q) = %v, want %v", c.out, got, c.want)
		}
	}
	if Classify(nil, "") != nil {
		t.Errorf("Classify(nil, _) should be nil")
	}
}

func TestRedactArgs(t *testing.T) {
	in := []string{"a", "-t7z", "-pSeCrEt", "-mhe=on", "-bsp1", "--", "out.7z", "in/"}
	got := redactArgs(in)
	for i, a := range got {
		if a == "-pSeCrEt" {
			t.Errorf("redactArgs failed at %d: %v", i, got)
		}
	}
	if got[2] != "-p***" {
		t.Errorf("expected redacted -p***, got %q", got[2])
	}
}

func TestFormatTypeFlag(t *testing.T) {
	cases := map[string]string{
		"zip": "zip", "7z": "7z", "tar": "tar",
		"tar.gz": "gzip", "tgz": "gzip", "gzip": "gzip",
		"tar.bz2": "bzip2", "bzip2": "bzip2",
		"tar.xz": "xz", "xz": "xz",
	}
	for in, want := range cases {
		got, err := formatTypeFlag(in)
		if err != nil || got != want {
			t.Errorf("formatTypeFlag(%q)=(%q,%v), want %q", in, got, err, want)
		}
	}
	if _, err := formatTypeFlag("rar"); err == nil {
		t.Errorf("formatTypeFlag(rar) should error")
	}
}

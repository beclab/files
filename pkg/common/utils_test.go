package common

import "testing"

// TestTrimShareId pins the contract documented on TrimShareId:
// an input is only truncated when it looks like
// "<36-char share UUID>_<known node name>", and is otherwise
// returned untouched. The previous implementation truncated
// purely by length, which let
// "<uuid>asdasd"-style URLs silently normalise back to the
// legitimate share id and hit the row -- this test guards
// against a regression.
func TestTrimShareId(t *testing.T) {
	const id = "00393f02-f939-4f08-871b-df1ab1a4e4c5" // 36 chars
	if len(id) != 36 {
		t.Fatalf("test fixture broken: id length is %d, want 36", len(id))
	}

	known := func(known ...string) func(string) bool {
		set := make(map[string]struct{}, len(known))
		for _, k := range known {
			set[k] = struct{}{}
		}
		return func(name string) bool {
			_, ok := set[name]
			return ok
		}
	}

	cases := []struct {
		name        string
		in          string
		isKnownNode func(string) bool
		want        string
	}{
		{
			name:        "bare uuid is unchanged",
			in:          id,
			isKnownNode: known("olares"),
			want:        id,
		},
		{
			name:        "decorated id with known node is stripped",
			in:          id + "_olares",
			isKnownNode: known("olares"),
			want:        id,
		},
		{
			name:        "decorated id with multi-underscore node name still works",
			in:          id + "_node_a",
			isKnownNode: known("node_a"),
			want:        id,
		},
		{
			name:        "decorated id with unknown node is left untouched",
			in:          id + "_olares",
			isKnownNode: known("other-node"),
			want:        id + "_olares",
		},
		// Regression: an earlier implementation truncated any
		// string longer than 36 chars, so "<uuid>asdasd" became
		// "<uuid>" and matched the real row. The new contract
		// requires the 37th byte to be '_' before trimming.
		{
			name:        "garbage suffix without underscore is left untouched",
			in:          id + "asdasd",
			isKnownNode: known("olares"),
			want:        id + "asdasd",
		},
		{
			name:        "decorated-looking id but suffix is unknown is left untouched",
			in:          id + "_bogus",
			isKnownNode: known("olares"),
			want:        id + "_bogus",
		},
		{
			name:        "shorter-than-uuid input is left untouched",
			in:          "too-short",
			isKnownNode: known("olares"),
			want:        "too-short",
		},
		{
			name:        "empty input is left untouched",
			in:          "",
			isKnownNode: known("olares"),
			want:        "",
		},
		// 37 chars total: a 36-char UUID + a trailing '_' with no
		// node name. There is no suffix to look up, so the value
		// is returned as-is rather than trimmed.
		{
			name:        "uuid plus lone underscore is left untouched",
			in:          id + "_",
			isKnownNode: known("olares"),
			want:        id + "_",
		},
		{
			name:        "nil isKnownNode treats every suffix as unknown",
			in:          id + "_olares",
			isKnownNode: nil,
			want:        id + "_olares",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := TrimShareId(tc.in, tc.isKnownNode)
			if got != tc.want {
				t.Fatalf("TrimShareId(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

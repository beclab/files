package database

import "testing"

func TestSanitizeOrderBy(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		want   string
		wantOk bool
	}{
		{"empty allowed", "", "", true},
		{"bare column allowed", "create_time", "create_time", true},
		{"qualified column allowed", "share_paths.id", "share_paths.id", true},
		{"underscored qualified", "share_members.path_id", "share_members.path_id", true},
		{"length wrapper allowed", "LENGTH(share_paths.path)", "LENGTH(share_paths.path)", true},
		{"identifier with digits allowed", "table1.col2", "table1.col2", true},

		{"two dots rejected", "a.b.c", "", false},
		{"trailing dot rejected", "table.", "", false},
		{"leading dot rejected", ".col", "", false},
		{"semicolon rejected", "id; DROP TABLE users--", "", false},
		{"injection via comma rejected", "id,1=1", "", false},
		{"whitespace rejected", "id ASC", "", false},
		{"function with args rejected", "COUNT(*)", "", false},
		{"length with two dots rejected", "LENGTH(a.b.c)", "", false},
		{"length with no parens rejected", "LENGTH share_paths.path", "", false},
		{"unicode rejected", "id\u200b", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := sanitizeOrderBy(tc.in)
			if got != tc.want || ok != tc.wantOk {
				t.Fatalf("sanitizeOrderBy(%q) = (%q, %v), want (%q, %v)",
					tc.in, got, ok, tc.want, tc.wantOk)
			}
		})
	}
}

func TestSanitizeOrderDirection(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "DESC"},
		{"ASC", "ASC"},
		{"asc", "ASC"},
		{"  desc  ", "DESC"},
		{"DESC NULLS LAST", "DESC NULLS LAST"},
		{"asc nulls first", "ASC NULLS FIRST"},
		{"desc nulls first", "DESC NULLS FIRST"},
		{"asc nulls last", "ASC NULLS LAST"},

		{"DESC; DROP TABLE users", "DESC"},
		{"RANDOM()", "DESC"},
		{"ASC, DESC", "DESC"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := sanitizeOrderDirection(tc.in); got != tc.want {
				t.Fatalf("sanitizeOrderDirection(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

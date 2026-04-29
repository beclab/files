package redact

import "testing"

func TestKey(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"", false},
		{"foo", false},
		{"FB_HOST", false},
		{"password", true},
		{"PASSWORD", true},
		{"db_password", true},
		{"AccessToken", true},
		{"MY_API_KEY", true},
		{"apikey", true},
		{"api_key", true},
		{"sessionCookie", true},
		{"credential", true},
		{"secret_value", true},
	}
	for _, c := range cases {
		if got := Key(c.name); got != c.want {
			t.Errorf("Key(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestValue(t *testing.T) {
	cases := []struct {
		key, value, want string
	}{
		{"FB_HOST", "example.com", "example.com"},
		{"FB_PASSWORD", "hunter2", "***"},
		{"FB_PASSWORD", "", ""},
		{"DB_TOKEN", "abc.def.ghi", "***"},
		{"AccessToken", "xyz", "***"},
		{"plain", "value", "value"},
		{"plain", "", ""},
	}
	for _, c := range cases {
		if got := Value(c.key, c.value); got != c.want {
			t.Errorf("Value(%q, %q) = %q, want %q", c.key, c.value, got, c.want)
		}
	}
}

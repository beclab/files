package posix

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"files/pkg/common"
	"files/pkg/integration"
	"files/pkg/models"
)

// TestCheckPermissionNonCommon covers the branches of
// PosixStorage.CheckPermission that do not require a populated
// integration service: owner-scoped resources are always admin, and
// drive/Common with no integration service falls back to read-only.
func TestCheckPermission(t *testing.T) {
	s := &PosixStorage{}

	ownerScoped := []*models.FileParam{
		{FileType: "drive", Extend: "Home", Path: "/a.txt"},
		{FileType: "drive", Extend: "Data", Path: "/a.txt"},
		{FileType: "cache", Extend: "node1", Path: "/a.txt"},
		{FileType: "external", Extend: "node1", Path: "/a.txt"},
	}
	for _, fp := range ownerScoped {
		lvl, err := s.CheckPermission(fp, "alice")
		if err != nil {
			t.Fatalf("CheckPermission(%s/%s) error: %v", fp.FileType, fp.Extend, err)
		}
		if lvl != models.LevelAdmin {
			t.Errorf("CheckPermission(%s/%s) = %v, want LevelAdmin", fp.FileType, fp.Extend, lvl)
		}
	}

	// drive/Common with no integration service available: a non-admin
	// (here, unknown) owner gets read-only.
	orig := integration.IntegrationService
	integration.IntegrationService = nil
	defer func() { integration.IntegrationService = orig }()

	common := &models.FileParam{FileType: "drive", Extend: "Common", Path: "/"}
	lvl, err := s.CheckPermission(common, "alice")
	if err != nil {
		t.Fatalf("CheckPermission(drive/Common) error: %v", err)
	}
	if lvl != models.LevelRead {
		t.Errorf("CheckPermission(drive/Common, no integration) = %v, want LevelRead", lvl)
	}
}

// TestCheckPermissionDriveCommonRole exercises the platform-role branch of
// drive/Common via the isPlatformAdmin seam: platform admins get LevelAdmin,
// everyone else LevelRead.
func TestCheckPermissionDriveCommonRole(t *testing.T) {
	s := &PosixStorage{}
	common := &models.FileParam{FileType: "drive", Extend: "Common", Path: "/docs"}

	saved := isPlatformAdmin
	t.Cleanup(func() { isPlatformAdmin = saved })

	isPlatformAdmin = func(owner string) bool { return owner == "boss" }

	cases := map[string]models.Level{
		"boss":  models.LevelAdmin,
		"alice": models.LevelRead,
	}
	for owner, want := range cases {
		lvl, err := s.CheckPermission(common, owner)
		if err != nil {
			t.Fatalf("CheckPermission(drive/Common, %s) error: %v", owner, err)
		}
		if lvl != want {
			t.Errorf("CheckPermission(drive/Common, %s) = %v, want %v", owner, lvl, want)
		}
	}
}

func TestProbeNilParam(t *testing.T) {
	s := &PosixStorage{}
	if err := s.ProbeExists(nil); err == nil {
		t.Fatalf("ProbeExists(nil) expected error")
	}
	if _, err := s.ProbeIsDir(nil); err == nil {
		t.Fatalf("ProbeIsDir(nil) expected error")
	}
	if err := s.ProbeWrite(nil); err == nil {
		t.Fatalf("ProbeWrite(nil) expected error")
	}
}

func TestProbeIsDirMissing(t *testing.T) {
	s := &PosixStorage{}
	fp := &models.FileParam{FileType: common.Drive, Extend: common.Common, Path: "/__no_such_path__/" + t.Name()}
	if _, err := s.ProbeIsDir(fp); err == nil {
		t.Fatalf("ProbeIsDir(missing) expected error")
	} else if !strings.Contains(err.Error(), "share target not found") && !strings.Contains(err.Error(), "stat share target") {
		t.Fatalf("ProbeIsDir(missing) message = %q", err.Error())
	}
}

func TestProbeParentDir(t *testing.T) {
	cases := map[string]string{
		"":              "/",
		"/":             "/",
		"foo":           "/",
		"/foo":          "/",
		"/foo/bar":      "/foo/",
		"/foo/bar/":     "/foo/",
		"/foo/bar/baz/": "/foo/bar/",
	}
	for in, want := range cases {
		got := probeParentDir(in)
		if got != want {
			t.Errorf("probeParentDir(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRemoteIsDirStatusCodes(t *testing.T) {
	saved := remoteLookupPodIP
	t.Cleanup(func() { remoteLookupPodIP = saved })

	cases := []struct {
		name    string
		status  int
		body    string
		wantErr bool
		wantDir bool
	}{
		{name: "200 dir", status: 200, body: `{"isDir": true}`, wantErr: false, wantDir: true},
		{name: "200 file", status: 200, body: `{"isDir": false}`, wantErr: false, wantDir: false},
		{name: "404", status: 404, body: "", wantErr: true},
		{name: "500", status: 500, body: "", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get(common.REQUEST_HEADER_OWNER); got != "alice" {
					t.Errorf("missing owner header, got %q", got)
				}
				w.WriteHeader(tc.status)
				if tc.body != "" {
					_, _ = w.Write([]byte(tc.body))
				}
			}))
			t.Cleanup(srv.Close)

			u, _ := url.Parse(srv.URL)
			remoteLookupPodIP = func(string) (string, error) { return u.Host, nil }

			fp := &models.FileParam{FileType: "external", Extend: "node-x", Path: "/x"}
			isDir, err := RemoteIsDir(fp, "alice")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got %v want %v", err, tc.wantErr)
			}
			if !tc.wantErr && isDir != tc.wantDir {
				t.Fatalf("isDir mismatch: got %v want %v", isDir, tc.wantDir)
			}
		})
	}
}

func TestRemoteExistsStatusCodes(t *testing.T) {
	saved := remoteLookupPodIP
	t.Cleanup(func() { remoteLookupPodIP = saved })

	cases := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "200", status: 200, wantErr: false},
		{name: "404", status: 404, wantErr: true},
		{name: "500", status: 500, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			t.Cleanup(srv.Close)

			u, _ := url.Parse(srv.URL)
			remoteLookupPodIP = func(string) (string, error) { return u.Host, nil }

			fp := &models.FileParam{FileType: "external", Extend: "node-x", Path: "/x"}
			err := RemoteExists(fp, "alice")
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got %v want %v", err, tc.wantErr)
			}
			if tc.wantErr {
				var statusErr *RemoteStatusError
				if !errors.As(err, &statusErr) {
					t.Fatalf("expected *RemoteStatusError, got %T: %v", err, err)
				}
			}
		})
	}
}

func TestRemoteProbeWriteStatusCodes(t *testing.T) {
	saved := remoteLookupPodIP
	t.Cleanup(func() { remoteLookupPodIP = saved })

	cases := []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "204 ok", status: 204, wantErr: false},
		{name: "403", status: 403, wantErr: true},
		{name: "500", status: 500, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("probe"); got != "write" {
					t.Errorf("missing probe=write query, got %q", got)
				}
				w.WriteHeader(tc.status)
			}))
			t.Cleanup(srv.Close)

			u, _ := url.Parse(srv.URL)
			remoteLookupPodIP = func(string) (string, error) { return u.Host, nil }

			fp := &models.FileParam{FileType: "external", Extend: "node-x", Path: "/x", Owner: "alice"}
			err := RemoteProbeWrite(fp)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got %v want %v", err, tc.wantErr)
			}
			if tc.wantErr && err != nil && !strings.Contains(err.Error(), "destination not writable") {
				t.Fatalf("expected 'destination not writable' in err, got %q", err.Error())
			}
		})
	}
}

func TestRemoteLookupFailure(t *testing.T) {
	saved := remoteLookupPodIP
	t.Cleanup(func() { remoteLookupPodIP = saved })
	remoteLookupPodIP = func(string) (string, error) { return "", errors.New("no pod") }

	if err := RemoteExists(&models.FileParam{Extend: "node-x"}, "alice"); err == nil {
		t.Fatalf("expected lookup error")
	}
	if _, err := RemoteIsDir(&models.FileParam{Extend: "node-x"}, "alice"); err == nil {
		t.Fatalf("expected lookup error")
	}
	if err := RemoteProbeWrite(&models.FileParam{Extend: "node-x"}); err == nil {
		t.Fatalf("expected lookup error")
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

type RouteCase struct {
	Method      string
	Pattern     string
	TestPath    string
	BodyFunc    func() io.Reader
	Headers     map[string]string
	Description string
	Category    string
	Stream      bool

	// Phase controls execution order within a category.
	// Lower phases run first. Default 0 = normal benchmark.
	// Negative phases = setup (not benchmarked, results stored for later use).
	// Phase 99 = teardown/cleanup (benchmarked but runs last).
	Phase int

	// DynPath, if set, is called at runtime to resolve the test path.
	// Used when a path depends on a value created during setup (e.g. repo ID).
	DynPath func() string

	// DynBody, if set, overrides BodyFunc at runtime.
	DynBody func() io.Reader

	// Skip marks a route as unsafe to execute. The route will not be
	// called but will appear in the report with the Recommendation.
	Skip       bool
	SkipReason string

	// Recommendation is a human-written timeout suggestion for routes
	// that cannot be safely benchmarked. Shown in the report as-is.
	Recommendation string
}

func (r RouteCase) ResolvePath() string {
	if r.DynPath != nil {
		return r.DynPath()
	}
	return r.TestPath
}

func (r RouteCase) ResolveBody() io.Reader {
	if r.DynBody != nil {
		return r.DynBody()
	}
	if r.BodyFunc != nil {
		return r.BodyFunc()
	}
	return nil
}

func jsonBody(v interface{}) func() io.Reader {
	return func() io.Reader {
		b, _ := json.Marshal(v)
		return bytes.NewReader(b)
	}
}

func stringBody(s string) func() io.Reader {
	return func() io.Reader { return strings.NewReader(s) }
}

// Runtime state shared between setup and benchmark phases.
// Populated by setup callbacks, consumed by DynPath/DynBody closures.
var (
	createdRepoID    string
	createdSharePath string
	createdTokenID   string
)

const benchDir = "apibench_testdir"
const benchFile = "apibench_test.txt"

func AllRoutes() []RouteCase {
	jsonCT := map[string]string{"Content-Type": "application/json"}

	return []RouteCase{
		// ────────────────────────────────────────────
		// Health / Ping
		// ────────────────────────────────────────────
		{Method: "GET", Pattern: "/ping", TestPath: "/ping", Description: "liveness ping", Category: "health"},
		{Method: "GET", Pattern: "/healthz", TestPath: "/healthz", Description: "health check (k8s)", Category: "health"},
		{Method: "GET", Pattern: "/health", TestPath: "/health", Description: "health check (docker)", Category: "health"},

		// ────────────────────────────────────────────
		// Resources (CRUD) — ordered: setup→read→modify→cleanup
		// ────────────────────────────────────────────
		// setup: create dir + file for later tests
		{Method: "POST", Pattern: "/api/resources/*path", TestPath: "/api/resources/drive/Documents/" + benchDir + "/", Description: "setup: create test directory", Category: "resources",
			Phase: -1, Headers: jsonCT, BodyFunc: jsonBody(map[string]string{"action": "mkdir"})},
		{Method: "PUT", Pattern: "/api/resources/*path", TestPath: "/api/resources/drive/Documents/" + benchDir + "/" + benchFile, Description: "setup: upload test file", Category: "resources",
			Phase: -1, BodyFunc: stringBody("apibench test content for benchmarking\n"), Stream: true},

		// benchmark reads
		{Method: "GET", Pattern: "/api/resources/*path", TestPath: "/api/resources/drive/", Description: "list root directory", Category: "resources"},
		{Method: "GET", Pattern: "/api/resources/*path (subdir)", TestPath: "/api/resources/drive/Documents/" + benchDir + "/", Description: "list test subdirectory", Category: "resources"},

		// non-idempotent writes: run once in setup, not N samples
		{Method: "POST", Pattern: "/api/resources/*path (mkdir)", TestPath: "/api/resources/drive/Documents/" + benchDir + "/subdir_bench/", Description: "setup: create sub-directory", Category: "resources",
			Phase: -1, Headers: jsonCT, BodyFunc: jsonBody(map[string]string{"action": "mkdir"})},
		{Method: "PATCH", Pattern: "/api/resources/*path", TestPath: "/api/resources/drive/Documents/" + benchDir + "/subdir_bench/", Description: "setup: rename resource", Category: "resources",
			Phase: -1, Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{"action": "rename", "destination": "/drive/Documents/" + benchDir + "/subdir_renamed/"})},
		{Method: "DELETE", Pattern: "/api/resources/*path (subdir)", TestPath: "/api/resources/drive/Documents/" + benchDir + "/subdir_renamed/", Description: "cleanup: delete renamed subdirectory", Category: "resources", Phase: 97},
		{Method: "PUT", Pattern: "/api/resources/*path (file)", TestPath: "/api/resources/drive/Documents/" + benchDir + "/bench_upload.txt", Description: "upload small file via PUT", Category: "resources",
			BodyFunc: stringBody("benchmark upload content"), Stream: true},

		// cleanup
		{Method: "DELETE", Pattern: "/api/resources/*path (cleanup)", TestPath: "/api/resources/drive/Documents/" + benchDir + "/", Description: "cleanup: delete test directory", Category: "resources", Phase: 99},

		// ────────────────────────────────────────────
		// Tree / Nodes
		// ────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/tree/*path", TestPath: "/api/tree/drive/", Description: "get directory tree", Category: "tree"},
		{Method: "GET", Pattern: "/api/nodes", TestPath: "/api/nodes/", Description: "list storage nodes", Category: "tree"},

		// ────────────────────────────────────────────
		// Raw / Preview
		// ────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/raw/*path", TestPath: "/api/raw/drive/Documents/" + benchDir + "/" + benchFile, Description: "raw file download", Category: "raw", Stream: true},
		{Method: "GET", Pattern: "/api/preview/*path", TestPath: "/api/preview/drive/Documents/" + benchDir + "/" + benchFile, Description: "preview file", Category: "preview", Stream: true},

		// ────────────────────────────────────────────
		// Upload
		// ────────────────────────────────────────────
		{Method: "GET", Pattern: "/upload/upload-link/:node", TestPath: "/upload/upload-link/drive/", Description: "get upload link", Category: "upload"},
		{Method: "GET", Pattern: "/upload/file-uploaded-bytes/:node", TestPath: "/upload/file-uploaded-bytes/drive/", Description: "query uploaded bytes", Category: "upload"},
		{Method: "POST", Pattern: "/upload/upload-link/:node/:uid", TestPath: "/upload/upload-link/drive/apibench-test-uid", Description: "upload file chunks (will 4xx, measures routing)", Category: "upload", Stream: true},
		{Method: "POST", Pattern: "/seafhttp/:upload/:uid", TestPath: "/seafhttp/upload/apibench-test-uid", Description: "sync upload chunks (will 4xx, measures routing)", Category: "upload", Stream: true},

		// ────────────────────────────────────────────
		// Paste / Task
		// ────────────────────────────────────────────
		{Method: "PATCH", Pattern: "/api/paste/:node", TestPath: "/api/paste/drive/", Description: "paste copy (will error without valid src, measures handler latency)", Category: "paste",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{
				"action": "copy", "source": "/drive/Documents/nonexistent_apibench", "destination": "/drive/Documents/nonexistent_apibench_dst",
			})},
		{Method: "GET", Pattern: "/api/task/:node", TestPath: "/api/task/drive/", Description: "list tasks", Category: "paste"},
		{Method: "POST", Pattern: "/api/task/:node", TestPath: "/api/task/drive/", Description: "pause/resume task (no active task, measures routing)", Category: "paste",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{"action": "pause", "id": "apibench-nonexistent"})},
		{Method: "DELETE", Pattern: "/api/task/:node", TestPath: "/api/task/drive/?id=apibench-nonexistent", Description: "cancel task (no active task, measures routing)", Category: "paste"},

		// ────────────────────────────────────────────
		// Search
		// ────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/search/check_directory/*path", TestPath: "/api/search/check_directory/drive/Documents/", Description: "check if directory exists", Category: "search"},
		{Method: "GET", Pattern: "/api/search/get_directory", TestPath: "/api/search/get_directory/", Description: "get directory listing for search", Category: "search"},
		{Method: "POST", Pattern: "/api/search/sync_search", TestPath: "/api/search/sync_search/", Description: "sync search", Category: "search",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]string{"q": "test"})},

		// ────────────────────────────────────────────
		// Share — full lifecycle: create→read→update→cleanup
		// ────────────────────────────────────────────
		// reads (no setup needed)
		{Method: "GET", Pattern: "/api/share/get_share", TestPath: "/api/share/get_share/", Description: "list external shares", Category: "share"},
		{Method: "GET", Pattern: "/api/share/get_share_internal_smb/*path", TestPath: "/api/share/get_share_internal_smb/drive/", Description: "get internal SMB share", Category: "share"},
		{Method: "GET", Pattern: "/api/share/share_member", TestPath: "/api/share/share_member/", Description: "list share members", Category: "share"},
		{Method: "GET", Pattern: "/api/share/share_path", TestPath: "/api/share/share_path/", Description: "list share paths", Category: "share"},
		{Method: "GET", Pattern: "/api/share/share_token", TestPath: "/api/share/share_token/", Description: "list share tokens", Category: "share"},
		{Method: "GET", Pattern: "/api/share/smb_share_user", TestPath: "/api/share/smb_share_user/", Description: "list SMB users", Category: "share"},

		// create share path — non-idempotent, run once in setup
		// NOTE: use "internal" share_type — "external" would create a K8s ClusterRole+Binding
		{Method: "POST", Pattern: "/api/share/share_path/*path", TestPath: "/api/share/share_path/drive/Documents/", Description: "setup: create share path (internal)", Category: "share",
			Phase: -1, Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{
				"share_type": "internal", "name": "apibench_share", "password": "bench123",
				"expire_in": 86400, "permission": 1,
			})},
		// get_token (validate share)
		{Method: "POST", Pattern: "/api/share/get_token", TestPath: "/api/share/get_token/", Description: "get share token", Category: "share",
			Headers: jsonCT, DynBody: func() io.Reader {
				if createdSharePath == "" {
					return jsonBody(map[string]string{"id": "none", "pass": "bench123"})()
				}
				return jsonBody(map[string]string{"id": createdSharePath, "pass": "bench123"})()
			}},

		// update operations
		{Method: "PUT", Pattern: "/api/share/share_path", TestPath: "/api/share/share_path/", Description: "update share path name", Category: "share",
			Headers: jsonCT, DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath, "name": "apibench_share_renamed"})()
			}},
		{Method: "PUT", Pattern: "/api/share/share_password", TestPath: "/api/share/share_password/", Description: "reset share password", Category: "share",
			Headers: jsonCT, DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath, "password": "newpass456"})()
			}},

		// SMB user lifecycle — create is non-idempotent, run once
		{Method: "POST", Pattern: "/api/share/smb_share_user", TestPath: "/api/share/smb_share_user/", Description: "setup: create SMB user", Category: "share",
			Phase: -1, Headers: jsonCT, BodyFunc: jsonBody(map[string]string{"user": "apibench_smb_user", "password": "smbpass123"})},
		{Method: "DELETE", Pattern: "/api/share/smb_share_user", TestPath: "/api/share/smb_share_user/", Description: "delete SMB user", Category: "share",
			Phase: 98, Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{"users": []string{"apibench_smb_user"}})},

		// SMB share member (best-effort, will 4xx if no SMB share)
		{Method: "POST", Pattern: "/api/share/smb_share_member", TestPath: "/api/share/smb_share_member/", Description: "modify SMB member (may 4xx)", Category: "share",
			Headers: jsonCT, DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath, "public_smb": false,
					"users": []map[string]interface{}{{"id": "apibench_smb_user", "permission": 1}}})()
			}},

		// Generate share token — non-idempotent (each call creates a new token)
		{Method: "POST", Pattern: "/api/share/share_token", TestPath: "/api/share/share_token/", Description: "setup: generate share token", Category: "share",
			Phase: -1, Headers: jsonCT, DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath, "expire_at": "2099-01-01T00:00:00Z"})()
			}},
		{Method: "DELETE", Pattern: "/api/share/share_token", TestPath: "/api/share/share_token/", Description: "revoke share token", Category: "share",
			Phase: 97, DynPath: func() string {
				if createdTokenID == "" {
					return "" // empty → skipped at runtime
				}
				return "/api/share/share_token/?token=" + createdTokenID
			}},

		// cleanup: delete share path — skip if setup never captured a path ID
		{Method: "DELETE", Pattern: "/api/share/share_path", TestPath: "/api/share/share_path/", Description: "cleanup: delete share path", Category: "share",
			Phase: 99, DynPath: func() string {
				if createdSharePath == "" {
					return "" // empty → skipped at runtime
				}
				return "/api/share/share_path/?path_ids=" + createdSharePath
			}},

		// share members — best-effort with potentially invalid IDs
		{Method: "POST", Pattern: "/api/share/share_member", TestPath: "/api/share/share_member/", Description: "add share member (may 4xx)", Category: "share",
			Headers: jsonCT, DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath,
					"share_members": []map[string]interface{}{{"share_member": "apibench_testmember", "permission": 1}}})()
			}},
		{Method: "PUT", Pattern: "/api/share/share_member", TestPath: "/api/share/share_member/", Description: "update share member permission (may 4xx)", Category: "share",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{
				"share_members": []map[string]interface{}{{"member_id": 99999, "permission": 2}},
			})},
		{Method: "DELETE", Pattern: "/api/share/share_member", TestPath: "/api/share/share_member/?member_ids=99999", Description: "remove share member (may 4xx)", Category: "share", Phase: 98},
		{Method: "PUT", Pattern: "/api/share/share_path/share_members", TestPath: "/api/share/share_path/share_members/", Description: "update share path members (may 4xx)", Category: "share",
			Headers: jsonCT, DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath,
					"share_members": []map[string]interface{}{{"share_member": "apibench_member", "permission": 1}}})()
			}},

		// ────────────────────────────────────────────
		// Users
		// ────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/users", TestPath: "/api/users/", Description: "list users", Category: "users"},

		// ────────────────────────────────────────────
		// Repos / Sync — full lifecycle via query params
		// ────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/repos", TestPath: "/api/repos/", Description: "list repos", Category: "repos"},
		{Method: "POST", Pattern: "/api/repos", TestPath: "/api/repos/?repoName=apibench_test_repo", Description: "setup: create repo", Category: "repos", Phase: -1},
		{Method: "PATCH", Pattern: "/api/repos", TestPath: "/api/repos/", Description: "rename repo", Category: "repos",
			DynPath: func() string {
				if createdRepoID == "" {
					return "" // empty → skipped at runtime
				}
				return "/api/repos/?repoId=" + createdRepoID + "&destination=apibench_renamed_repo"
			}},
		{Method: "GET", Pattern: "/api/repos/:repo_id/download-info", TestPath: "/api/repos/", Description: "get repo download info", Category: "repos",
			DynPath: func() string {
				if createdRepoID == "" {
					return "" // empty → skipped at runtime
				}
				return "/api/repos/" + createdRepoID + "/download-info/"
			}},
		{Method: "DELETE", Pattern: "/api/repos", TestPath: "/api/repos/", Description: "cleanup: delete repo", Category: "repos",
			Phase: 99, DynPath: func() string {
				if createdRepoID == "" {
					return "" // empty → skipped at runtime
				}
				return "/api/repos/?repoId=" + createdRepoID
			}},
		{Method: "GET", Pattern: "/api/sync/account/info", TestPath: "/api/sync/account/info/", Description: "sync account info", Category: "repos"},

		// ────────────────────────────────────────────
		// Permission
		// ────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/permission/*path", TestPath: "/api/permission/drive/Documents/", Description: "get permission", Category: "permission"},
		{Method: "PUT", Pattern: "/api/permission/*path", TestPath: "/api/permission/drive/Documents/" + benchDir + "/?uid=1000&recursive=0", Description: "set permission on test dir (uid 1000)", Category: "permission"},

		// ────────────────────────────────────────────
		// MD5
		// ────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/md5/*path", TestPath: "/api/md5/drive/Documents/" + benchDir + "/" + benchFile, Description: "compute file MD5", Category: "md5"},

		// ────────────────────────────────────────────
		// External
		// ────────────────────────────────────────────
		{Method: "GET", Pattern: "/api/accounts", TestPath: "/api/accounts", Description: "list cloud accounts", Category: "external"},
		{Method: "POST", Pattern: "/api/mount/:node", TestPath: "/api/mount/drive/?external_type=smb", Description: "mount SMB (will fail without valid SMB server, measures routing)", Category: "external",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{"smbPath": "//apibench-nonexistent/share", "user": "test", "password": "test"})},
		{Method: "GET", Pattern: "/api/mounted/:node", TestPath: "/api/mounted/drive/", Description: "list mounted drives", Category: "external"},
		{Method: "POST", Pattern: "/api/unmount/*path", TestPath: "/api/unmount/drive/apibench-nonexistent/?external_type=smb", Description: "unmount (no such mount, measures routing)", Category: "external"},
		{Method: "GET", Pattern: "/api/smb_history/:node", TestPath: "/api/smb_history/drive/", Description: "get SMB history", Category: "external"},
		{Method: "PUT", Pattern: "/api/smb_history/:node", TestPath: "/api/smb_history/drive/", Description: "update SMB history", Category: "external",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{})},
		{Method: "DELETE", Pattern: "/api/smb_history/:node", TestPath: "/api/smb_history/drive/", Description: "delete SMB history", Category: "external"},

		// ────────────────────────────────────────────
		// Callback — SKIP: these are internal user lifecycle events.
		// callback/create creates a Seafile user + default library;
		// callback/delete REMOVES a Seafile user and all their shares.
		// Both mutate Seafile state that may be shared across nodes.
		// ────────────────────────────────────────────
		{Method: "POST", Pattern: "/callback/create", TestPath: "/callback/create", Description: "callback create (creates Seafile user)", Category: "callback",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]string{"name": "apibench_callback_test"}),
			Skip: true, SkipReason: "creates real Seafile user + library; affects shared Seafile DB",
			Recommendation: "Call chain: HandleCallbackCreate → CreateUser → ListAllUsers " +
				"(3 Ccnet RPCs + O(N) Redis HGetAll per user for email mapping) → " +
				"SaveUser (GetEmailuser + AddEmailuser, 2 Ccnet RPCs) → " +
				"CreateDefaultLibrary (1 Seafile CreateRepo RPC + GetSystemDefaultRepoId + " +
				"ListDirByPath + sequential CopyFile per template entry). " +
				"All I/O is over Unix domain socket with no application-level timeout. " +
				"Dominant cost: ListAllUsers scales with user count (Redis round-trips); " +
				"CopyFile is sequential per template file. " +
				"For a fresh system (~1-5 users, 3-5 template files): ~500-1500ms. " +
				"For a system with 50+ users: could reach 2-5s due to Redis HGetAll loop. " +
				"Comparable: POST /api/repos (benchmarked) does CreateRepo only (~200-500ms), " +
				"but callback/create adds user creation + template copies on top. " +
				"Suggest timeout: 10s (accounts for large user lists and template copies)."},
		{Method: "POST", Pattern: "/callback/delete", TestPath: "/callback/delete", Description: "callback delete (removes Seafile user)", Category: "callback",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]string{"name": "apibench_callback_test"}),
			Skip: true, SkipReason: "DELETES real Seafile user + all shares; affects shared Seafile DB",
			Recommendation: "Call chain: RemoveUserRelativeAdjustShare (Postgres tx: " +
				"QuerySharePath + per-path DeleteSharePathRelations + per-sync-share " +
				"HandleDeleteDirSharedItems which does GetRepo/GetDirIdByPath/GetRepoOwner/" +
				"RemoveShare RPCs sequentially + DeleteShareMember + Commit) → " +
				"HandleCallbackDelete → RemoveUser (GetEmailuser + GetOwnedRepoList + " +
				"sequential RemoveRepo per owned repo + GetShareInRepoList + " +
				"sequential RemoveShare per inbound share + DeleteRepoTokensByEmail + " +
				"RemoveGroupUser + RemoveEmailuser). " +
				"All Seafile RPCs are Unix socket, no timeout. GetRepo/GetDirIdByPath " +
				"use rpcWithRetry (up to 4 attempts, 200ms backoff). " +
				"Dominant cost: scales with owned repos + inbound shares " +
				"(each is a sequential RPC). " +
				"For a user with 1-3 repos, few shares: ~500-1500ms. " +
				"For a user with 10+ repos and many shares: could reach 5-10s. " +
				"Comparable: DELETE /api/repos (benchmarked) does similar share cleanup " +
				"+ single repo delete (~300-800ms), but callback/delete adds " +
				"full user deletion across ALL repos. " +
				"Suggest timeout: 15s (accounts for heavy users with many repos/shares)."},

		// ────────────────────────────────────────────
		// Media
		// ────────────────────────────────────────────
		{Method: "GET", Pattern: "/system/configuration/:key", TestPath: "/system/configuration/encoding", Description: "get media config", Category: "media"},
		{Method: "POST", Pattern: "/system/configuration/:key", TestPath: "/system/configuration/encoding", Description: "update media config", Category: "media",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{}),
			Skip: true, SkipReason: "sends empty JSON body which may corrupt encoding config",
			Recommendation: "Call chain: UpdateNamedConfiguration → JSON unmarshal (CPU) → " +
				"Validate → GetConfiguration → GetConfigurationFromConfigMap " +
				"(K8s API: ConfigMaps.Get, ~10-50ms intra-cluster) → " +
				"SaveConfigurationToConfigMap (serialize JSON + WriteConfigMap: " +
				"ConfigMaps.Get + ConfigMaps.Update, ~20-80ms total). " +
				"All K8s calls use client-go REST with context.Background() " +
				"and no per-request deadline. " +
				"GET variant (benchmarked) does the read path only (~10-50ms). " +
				"POST adds one extra ConfigMap write (~10-30ms on top). " +
				"Total expected: 30-100ms under normal K8s API server load. " +
				"Suggest timeout: 5s (K8s API can spike under etcd pressure)."},
		{Method: "GET", Pattern: "/videos/master.m3u8", TestPath: "/videos/master.m3u8", Description: "master HLS playlist (no item, measures routing)", Category: "media", Stream: true},
		{Method: "GET", Pattern: "/videos/:node", TestPath: "/videos/apibench-test-node", Description: "custom play controller (no item, measures routing)", Category: "media", Stream: true},
		{Method: "GET", Pattern: "/videos/:node/main.m3u8", TestPath: "/videos/apibench-test-node/main.m3u8", Description: "variant HLS playlist (no item, measures routing)", Category: "media", Stream: true},
		{Method: "GET", Pattern: "/videos/:node/hls1/:playlistId/:filename", TestPath: "/videos/apibench-test-node/hls1/0/segment0.ts", Description: "HLS video segment (no session, measures routing)", Category: "media", Stream: true},
	}
}

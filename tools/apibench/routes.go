package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math/rand"
	"mime/multipart"
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

	Phase   int
	DynPath func() string
	DynBody func() io.Reader

	Skip           bool
	SkipReason     string
	Recommendation string

	CurrentTimeout string // current nginx proxy_read_timeout for this route
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

// sizedReader wraps a reader and records its total size for reporting.
type sizedReader struct {
	io.Reader
	size int64
}

// randomBody returns a func that produces a body of exactly sizeBytes random data,
// wrapped in a sizedReader so doRequest can report ReqSize.
func randomBody(sizeBytes int) func() io.Reader {
	return func() io.Reader {
		buf := make([]byte, sizeBytes)
		rand.Read(buf)
		return &sizedReader{Reader: bytes.NewReader(buf), size: int64(sizeBytes)}
	}
}

// buildMultipartChunk constructs a real multipart/form-data body
// that mimics a resumable upload chunk with all required fields.
// Returns the body reader factory and the Content-Type header value.
func buildMultipartChunk(filename string, chunkSizeMB int, parentDir string) (func() io.Reader, string) {
	chunkBytes := chunkSizeMB * 1024 * 1024

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	w.WriteField("resumableChunkNumber", "1")
	w.WriteField("resumableChunkSize", fmt.Sprintf("%d", chunkBytes))
	w.WriteField("resumableCurrentChunkSize", fmt.Sprintf("%d", chunkBytes))
	w.WriteField("resumableTotalSize", fmt.Sprintf("%d", chunkBytes))
	w.WriteField("resumableType", "application/octet-stream")
	w.WriteField("resumableIdentifier", fmt.Sprintf("apibench-%d-%s", chunkBytes, filename))
	w.WriteField("resumableFilename", filename)
	w.WriteField("resumableRelativePath", filename)
	w.WriteField("resumableTotalChunks", "1")
	w.WriteField("parent_dir", parentDir)

	part, _ := w.CreateFormFile("file", filename)
	chunk := make([]byte, chunkBytes)
	rand.Read(chunk)
	part.Write(chunk)

	w.Close()
	contentType := w.FormDataContentType()
	frozen := buf.Bytes()

	fn := func() io.Reader {
		return &sizedReader{Reader: bytes.NewReader(frozen), size: int64(len(frozen))}
	}
	return fn, contentType
}

var (
	createdRepoID    string
	createdSharePath string
	createdTokenID   string
)

func pngBody() func() io.Reader {
	const w, h = 1920, 1080
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * 37) % 256),
				G: uint8((y * 53) % 256),
				B: uint8(((x + y) * 97) % 256),
				A: 255,
			})
		}
	}
	png.Encode(&buf, img)
	data := buf.Bytes()
	return func() io.Reader {
		return &sizedReader{Reader: bytes.NewReader(data), size: int64(len(data))}
	}
}

const benchDir = "apibench_testdir"
const benchFile = "apibench_test.txt"
const benchPNG = "apibench_test.png"
const bigDirCount = 200

func AllRoutes(cfg Config) []RouteCase {
	jsonCT := map[string]string{"Content-Type": "application/json"}

	routes := []RouteCase{
		// Pre-cleanup: remove residue from previous interrupted runs (best-effort)
		{Method: "DELETE", Pattern: "/api/resources/*path (pre-cleanup dir)", TestPath: "/api/resources/drive/Home/Documents/",
			Description: "pre-cleanup: remove leftover test directory (may 4xx, will fail if absent)", Category: "resources", Phase: -10,
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{"dirents": []string{"/" + benchDir}})},

		// Health / Ping
		{Method: "GET", Pattern: "/ping", TestPath: "/ping", Description: "liveness ping", Category: "health"},
		{Method: "GET", Pattern: "/healthz", TestPath: "/healthz", Description: "health check (k8s)", Category: "health"},
		{Method: "GET", Pattern: "/health", TestPath: "/health", Description: "health check (docker)", Category: "health"},

		// Resources (CRUD)
		{Method: "POST", Pattern: "/api/resources/*path", TestPath: "/api/resources/drive/Home/Documents/" + benchDir + "/", Description: "setup: create test directory", Category: "resources",
			Phase: -3, Headers: jsonCT, BodyFunc: jsonBody(map[string]string{"action": "mkdir"})},
		{Method: "POST", Pattern: "/api/resources/*path (create file)", TestPath: "/api/resources/drive/Home/Documents/" + benchDir + "/" + benchFile, Description: "setup: create test file", Category: "resources",
			Phase: -1},
		{Method: "POST", Pattern: "/api/resources/*path (create png)", TestPath: "/api/resources/drive/Home/Documents/" + benchDir + "/" + benchPNG, Description: "setup: create preview test image (empty)", Category: "resources",
			Phase: -1},
		{Method: "PUT", Pattern: "/api/resources/*path (write png)", TestPath: "/api/resources/drive/Home/Documents/" + benchDir + "/" + benchPNG, Description: "setup: write PNG content", Category: "resources",
			Phase: -1, BodyFunc: pngBody(), Stream: true},

		{Method: "GET", Pattern: "/api/resources/*path", TestPath: "/api/resources/drive/Home/", Description: "list root directory", Category: "resources"},
		{Method: "GET", Pattern: "/api/resources/*path (subdir)", TestPath: "/api/resources/drive/Home/Documents/" + benchDir + "/", Description: "list test subdirectory", Category: "resources"},

		{Method: "DELETE", Pattern: "/api/resources/*path (pre-cleanup subdirs)", TestPath: "/api/resources/drive/Home/Documents/" + benchDir + "/", Description: "setup: clean subdirs before rename (may 4xx, will fail if absent)", Category: "resources",
			Phase: -1, Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{"dirents": []string{"/subdir_bench", "/subdir_renamed"}})},
		{Method: "POST", Pattern: "/api/resources/*path (mkdir)", TestPath: "/api/resources/drive/Home/Documents/" + benchDir + "/subdir_bench/", Description: "setup: create sub-directory for rename test", Category: "resources",
			Phase: -1, Headers: jsonCT, BodyFunc: jsonBody(map[string]string{"action": "mkdir"})},
		{Method: "PATCH", Pattern: "/api/resources/*path", TestPath: "/api/resources/drive/Home/Documents/" + benchDir + "/subdir_bench/?destination=subdir_renamed", Description: "setup: rename resource", Category: "resources",
			Phase: -1},
		{Method: "DELETE", Pattern: "/api/resources/*path (subdir)", TestPath: "/api/resources/drive/Home/Documents/" + benchDir + "/", Description: "cleanup: delete renamed subdirectory", Category: "resources", Phase: 97,
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{"dirents": []string{"/subdir_bench", "/subdir_renamed"}})},

		// Tree / Nodes
		{Method: "GET", Pattern: "/api/tree/*path", TestPath: "/api/tree/drive/Home/", Description: "get directory tree", Category: "tree"},
		{Method: "GET", Pattern: "/api/nodes", TestPath: "/api/nodes/", Description: "list storage nodes", Category: "tree"},

		// Raw / Preview
		{Method: "GET", Pattern: "/api/raw/*path", TestPath: "/api/raw/drive/Home/Documents/" + benchDir + "/" + benchFile, Description: "raw file download", Category: "raw", Stream: true},
		{Method: "GET", Pattern: "/api/preview/*path (thumb)", TestPath: "/api/preview/drive/Home/Documents/" + benchDir + "/" + benchPNG + "?size=thumb", Description: "preview image (thumbnail)", Category: "preview", Stream: true},
		{Method: "GET", Pattern: "/api/preview/*path (big)", TestPath: "/api/preview/drive/Home/Documents/" + benchDir + "/" + benchPNG + "?size=big", Description: "preview image (big)", Category: "preview", Stream: true},

		// Upload: get-link and query
		{Method: "GET", Pattern: "/upload/upload-link/:node", TestPath: "/upload/upload-link/drive/?file_path=/drive/Home/Documents/" + benchDir + "/" + benchFile + "&from=drive", Description: "get upload link", Category: "upload"},
		{Method: "GET", Pattern: "/upload/file-uploaded-bytes/:node", TestPath: "/upload/file-uploaded-bytes/drive/?parent_dir=/drive/Home/Documents/" + benchDir + "/&file_name=" + benchFile, Description: "query uploaded bytes", Category: "upload"},

		// Paste / Task
		{Method: "PATCH", Pattern: "/api/paste/:node", TestPath: "/api/paste/drive/", Description: "paste copy (will error without valid src, measures handler latency)", Category: "paste",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{
				"action": "copy", "source": "/drive/Home/Documents/nonexistent_apibench", "destination": "/drive/Home/Documents/nonexistent_apibench_dst",
			})},
		{Method: "GET", Pattern: "/api/task/:node", TestPath: "/api/task/drive/", Description: "list tasks", Category: "paste"},
		{Method: "POST", Pattern: "/api/task/:node", TestPath: "/api/task/drive/", Description: "pause/resume task (no active task, measures routing)", Category: "paste",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{"action": "pause", "id": "apibench-nonexistent"})},
		{Method: "DELETE", Pattern: "/api/task/:node", TestPath: "/api/task/drive/?id=apibench-nonexistent", Description: "cancel task (no active task, measures routing)", Category: "paste"},

		// Search
		{Method: "GET", Pattern: "/api/search/check_directory/*path", TestPath: "/api/search/check_directory/drive/Home/Documents/", Description: "check if directory exists", Category: "search"},
		{Method: "GET", Pattern: "/api/search/get_directory", TestPath: "/api/search/get_directory/", Description: "get directory listing for search", Category: "search"},
		{Method: "POST", Pattern: "/api/search/sync_search", TestPath: "/api/search/sync_search/", Description: "sync search", Category: "search",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]string{"q": "test"})},

		// Share lifecycle
		{Method: "GET", Pattern: "/api/share/get_share", TestPath: "/api/share/get_share/", Description: "get external share by token (may 4xx)", Category: "share",
			DynPath: func() string {
				if createdSharePath == "" || createdTokenID == "" {
					return ""
				}
				return "/api/share/get_share/?path_id=" + createdSharePath + "&token=" + createdTokenID
			}},
		{Method: "GET", Pattern: "/api/share/get_share_internal_smb/*path", TestPath: "/api/share/get_share_internal_smb/drive/Home/", Description: "get internal SMB share", Category: "share"},
		{Method: "GET", Pattern: "/api/share/share_member", TestPath: "/api/share/share_member/", Description: "list share members", Category: "share"},
		{Method: "GET", Pattern: "/api/share/share_path", TestPath: "/api/share/share_path/", Description: "list share paths", Category: "share"},
		{Method: "GET", Pattern: "/api/share/share_token", TestPath: "/api/share/share_token/", Description: "list share tokens", Category: "share"},
		{Method: "GET", Pattern: "/api/share/smb_share_user", TestPath: "/api/share/smb_share_user/", Description: "list SMB users", Category: "share"},

		{Method: "POST", Pattern: "/api/share/share_path/*path", TestPath: "/api/share/share_path/drive/Home/Documents/", Description: "setup: create share path (internal)", Category: "share",
			Phase: -1, Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{
				"share_type": "internal", "name": "apibench_share", "password": "bench123",
				"expire_in": 86400, "permission": 1,
			})},
		{Method: "POST", Pattern: "/api/share/get_token", TestPath: "/api/share/get_token/", Description: "get share token", Category: "share",
			Headers: jsonCT,
			DynPath: func() string {
				if createdSharePath == "" {
					return ""
				}
				return "/api/share/get_token/"
			},
			DynBody: func() io.Reader {
				return jsonBody(map[string]string{"id": createdSharePath, "pass": "bench123"})()
			}},

		{Method: "PUT", Pattern: "/api/share/share_path", TestPath: "/api/share/share_path/", Description: "update share path name", Category: "share",
			Headers: jsonCT,
			DynPath: func() string {
				if createdSharePath == "" {
					return ""
				}
				return "/api/share/share_path/"
			},
			DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath, "name": "apibench_share_renamed"})()
			}},
		{Method: "PUT", Pattern: "/api/share/share_password", TestPath: "/api/share/share_password/", Description: "reset share password", Category: "share",
			Headers: jsonCT,
			DynPath: func() string {
				if createdSharePath == "" {
					return ""
				}
				return "/api/share/share_password/"
			},
			DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath, "password": "newpass456"})()
			}},

		{Method: "POST", Pattern: "/api/share/smb_share_user", TestPath: "/api/share/smb_share_user/", Description: "setup: create SMB user", Category: "share",
			Phase: -1, Headers: jsonCT, BodyFunc: jsonBody(map[string]string{"user": "apibench_smb_user", "password": "smbpass123"})},
		{Method: "DELETE", Pattern: "/api/share/smb_share_user", TestPath: "/api/share/smb_share_user/", Description: "delete SMB user", Category: "share",
			Phase: 98, Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{"users": []string{"apibench_smb_user"}})},

		{Method: "POST", Pattern: "/api/share/smb_share_member", TestPath: "/api/share/smb_share_member/", Description: "modify SMB member (may 4xx)", Category: "share",
			Headers: jsonCT,
			DynPath: func() string {
				if createdSharePath == "" {
					return ""
				}
				return "/api/share/smb_share_member/"
			},
			DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath, "public_smb": false,
					"users": []map[string]interface{}{{"id": "apibench_smb_user", "permission": 1}}})()
			}},

		{Method: "POST", Pattern: "/api/share/share_token", TestPath: "/api/share/share_token/", Description: "setup: generate share token", Category: "share",
			Phase: -1, Headers: jsonCT,
			DynPath: func() string {
				if createdSharePath == "" {
					return ""
				}
				return "/api/share/share_token/"
			},
			DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath, "expire_at": "2099-01-01T00:00:00Z"})()
			}},
		{Method: "DELETE", Pattern: "/api/share/share_token", TestPath: "/api/share/share_token/", Description: "revoke share token", Category: "share",
			Phase: 97, DynPath: func() string {
				if createdTokenID == "" {
					return ""
				}
				return "/api/share/share_token/?token=" + createdTokenID
			}},

		{Method: "DELETE", Pattern: "/api/share/share_path", TestPath: "/api/share/share_path/", Description: "cleanup: delete share path", Category: "share",
			Phase: 99, DynPath: func() string {
				if createdSharePath == "" {
					return ""
				}
				return "/api/share/share_path/?path_ids=" + createdSharePath
			}},

		{Method: "POST", Pattern: "/api/share/share_member", TestPath: "/api/share/share_member/", Description: "add share member (may 4xx)", Category: "share",
			Headers: jsonCT,
			DynPath: func() string {
				if createdSharePath == "" {
					return ""
				}
				return "/api/share/share_member/"
			},
			DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath,
					"share_members": []map[string]interface{}{{"share_member": "apibench_testmember", "permission": 1}}})()
			}},
		{Method: "PUT", Pattern: "/api/share/share_member", TestPath: "/api/share/share_member/", Description: "update share member permission (may 4xx)", Category: "share",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{
				"share_members": []map[string]interface{}{{"member_id": 99999, "permission": 2}},
			})},
		{Method: "DELETE", Pattern: "/api/share/share_member", TestPath: "/api/share/share_member/?member_ids=99999", Description: "remove share member (may 4xx)", Category: "share", Phase: 98},
		{Method: "PUT", Pattern: "/api/share/share_path/share_members", TestPath: "/api/share/share_path/share_members/", Description: "update share path members (may 4xx)", Category: "share",
			Headers: jsonCT,
			DynPath: func() string {
				if createdSharePath == "" {
					return ""
				}
				return "/api/share/share_path/share_members/"
			},
			DynBody: func() io.Reader {
				return jsonBody(map[string]interface{}{"path_id": createdSharePath,
					"share_members": []map[string]interface{}{{"share_member": "apibench_member", "permission": 1}}})()
			}},

		// Users
		{Method: "GET", Pattern: "/api/users", TestPath: "/api/users/", Description: "list users", Category: "users"},

		// Repos / Sync
		{Method: "GET", Pattern: "/api/repos", TestPath: "/api/repos/", Description: "list repos", Category: "repos"},
		{Method: "POST", Pattern: "/api/repos", TestPath: "/api/repos/?repoName=apibench_test_repo", Description: "setup: create repo", Category: "repos", Phase: -1},
		{Method: "PATCH", Pattern: "/api/repos", TestPath: "/api/repos/", Description: "rename repo", Category: "repos",
			DynPath: func() string {
				if createdRepoID == "" {
					return ""
				}
				return "/api/repos/?repoId=" + createdRepoID + "&destination=apibench_renamed_repo"
			}},
		{Method: "GET", Pattern: "/api/repos/:repo_id/download-info", TestPath: "/api/repos/", Description: "get repo download info", Category: "repos",
			DynPath: func() string {
				if createdRepoID == "" {
					return ""
				}
				return "/api/repos/" + createdRepoID + "/download-info/"
			}},
		{Method: "DELETE", Pattern: "/api/repos", TestPath: "/api/repos/", Description: "cleanup: delete repo", Category: "repos",
			Phase: 99, DynPath: func() string {
				if createdRepoID == "" {
					return ""
				}
				return "/api/repos/?repoId=" + createdRepoID
			}},
		{Method: "GET", Pattern: "/api/sync/account/info", TestPath: "/api/sync/account/info/", Description: "sync account info", Category: "repos"},

		// Permission
		{Method: "GET", Pattern: "/api/permission/*path", TestPath: "/api/permission/drive/Home/Documents/", Description: "get permission", Category: "permission"},
		{Method: "PUT", Pattern: "/api/permission/*path", TestPath: "/api/permission/drive/Home/Documents/" + benchDir + "/?uid=1000&recursive=0", Description: "set permission on test dir (uid 1000)", Category: "permission"},

		// MD5
		{Method: "GET", Pattern: "/api/md5/*path", TestPath: "/api/md5/drive/Home/Documents/" + benchDir + "/" + benchFile, Description: "compute file MD5", Category: "md5",
			Skip: true, SkipReason: "latency = network + O(file_size) MD5 computation; single-file benchmark is not representative",
			Recommendation: "MD5 computation is CPU-bound and linear in file size. " +
				"The handler reads the entire file into a hash.Sum and returns the hex digest. " +
				"Approximate throughput: ~400-600 MB/s on modern CPUs (single-core md5). " +
				"For a 100MB file: ~200ms compute + network overhead. " +
				"For a 1GB file: ~2s compute + network overhead. " +
				"Timeout should be set as a function of max expected file size: " +
				"suggest base 2s + 3s per GB, i.e. timeout = 2s + ceil(fileSize_GB) * 3s. " +
				"For typical max file size 4GB: suggest timeout 15s."},

		// External
		{Method: "GET", Pattern: "/api/accounts", TestPath: "/api/accounts", Description: "list cloud accounts", Category: "external"},
		{Method: "POST", Pattern: "/api/mount/:node", TestPath: "/api/mount/drive/?external_type=smb", Description: "mount SMB (will fail without valid SMB server, measures routing)", Category: "external",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{"smbPath": "//apibench-nonexistent/share", "user": "test", "password": "test"})},
		{Method: "GET", Pattern: "/api/mounted/:node", TestPath: "/api/mounted/drive/", Description: "list mounted drives", Category: "external"},
		{Method: "POST", Pattern: "/api/unmount/*path", TestPath: "/api/unmount/drive/apibench-nonexistent/?external_type=smb", Description: "unmount (no such mount, measures routing)", Category: "external"},
		{Method: "GET", Pattern: "/api/smb_history/:node", TestPath: "/api/smb_history/drive/", Description: "get SMB history", Category: "external"},
		{Method: "PUT", Pattern: "/api/smb_history/:node", TestPath: "/api/smb_history/drive/", Description: "update SMB history (may 4xx, empty body)", Category: "external",
			Headers: jsonCT, BodyFunc: jsonBody(map[string]interface{}{})},
		{Method: "DELETE", Pattern: "/api/smb_history/:node", TestPath: "/api/smb_history/drive/", Description: "delete SMB history (may 4xx, no id)", Category: "external"},

		// Callback
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

		// Media
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

	// PUT upload with realistic body sizes
	for _, sizeMB := range cfg.UploadSizes {
		sizeBytes := sizeMB * 1024 * 1024
		testPath := fmt.Sprintf("/api/resources/drive/Home/Documents/%s/bench_upload_%dmb.bin", benchDir, sizeMB)
		// setup: create file first (PUT = edit, requires existing file)
		routes = append(routes, RouteCase{
			Method:      "POST",
			Pattern:     fmt.Sprintf("/api/resources/*path (PUT %dMB setup)", sizeMB),
			TestPath:    testPath,
			Description: fmt.Sprintf("setup: create %dMB upload target", sizeMB),
			Category:    "resources",
			Phase:       -1,
		})
		routes = append(routes, RouteCase{
			Method:      "PUT",
			Pattern:     fmt.Sprintf("/api/resources/*path (PUT %dMB)", sizeMB),
			TestPath:    testPath,
			Description: fmt.Sprintf("upload %dMB file via PUT", sizeMB),
			Category:    "resources",
			Stream:      true,
			BodyFunc:    randomBody(sizeBytes),
		})
		// cleanup the uploaded file
		fname := fmt.Sprintf("bench_upload_%dmb.bin", sizeMB)
		routes = append(routes, RouteCase{
			Method:      "DELETE",
			Pattern:     fmt.Sprintf("/api/resources/*path (PUT %dMB cleanup)", sizeMB),
			TestPath:    fmt.Sprintf("/api/resources/drive/Home/Documents/%s/", benchDir),
			Description: fmt.Sprintf("cleanup: delete %dMB upload", sizeMB),
			Category:    "resources",
			Phase:       98,
			Headers:     jsonCT,
			BodyFunc:    jsonBody(map[string]interface{}{"dirents": []string{"/" + fname}}),
		})
	}

	// Multipart upload chunk routes (realistic upload simulation)
	for _, sizeMB := range cfg.UploadSizes {
		posixFilename := fmt.Sprintf("apibench_chunk_posix_%dmb.bin", sizeMB)
		posixBodyFn, posixCT := buildMultipartChunk(posixFilename, sizeMB, "/drive/Home/Documents/"+benchDir)

		routes = append(routes, RouteCase{
			Method:      "POST",
			Pattern:     fmt.Sprintf("/upload/upload-link/:node/:uid (%dMB chunk)", sizeMB),
			TestPath:    "/upload/upload-link/drive/apibench-upload-" + fmt.Sprintf("%dmb", sizeMB),
			Description: fmt.Sprintf("upload %dMB multipart chunk (posix, may 500 response unmarshal)", sizeMB),
			Category:    "upload",
			Stream:      true,
			Headers:     map[string]string{"Content-Type": posixCT},
			BodyFunc:    posixBodyFn,
		})
		routes = append(routes, RouteCase{
			Method:      "POST",
			Pattern:     fmt.Sprintf("/seafhttp/:upload/:uid (%dMB chunk)", sizeMB),
			TestPath:    "/seafhttp/upload/apibench-upload-" + fmt.Sprintf("%dmb", sizeMB),
			Description: fmt.Sprintf("upload %dMB multipart chunk (sync)", sizeMB),
			Category:    "upload",
			Stream:      true,
			Skip:        true,
			SkipReason:  "Sync upload differs only in the final commit API; constructing its request body requires calling the same upload-link and chunked-upload endpoints already benchmarked under Posix, so it is skipped to avoid redundant measurement",
			Recommendation: fmt.Sprintf("Sync upload shares the same nginx location /seafhttp/ (proxy_read_timeout 600s) and proxies to seafile:8082. "+
				"The actual upload I/O path is similar to Posix but goes through Seafile's file-server. "+
				"Use the Posix %dMB upload measurement as a baseline; Sync uploads may be slightly slower due to the extra Seafile hop. "+
				"Suggest timeout: same as Posix upload or higher (at least 30s per MB for worst-case networks).", sizeMB),
		})
	}

	// big-dir setup and listing
	if cfg.BigDir {
		for i := 0; i < bigDirCount; i++ {
			fname := fmt.Sprintf("bigdir_file_%04d.txt", i)
			routes = append(routes, RouteCase{
				Method:      "POST",
				Pattern:     fmt.Sprintf("/api/resources/*path (bigdir setup %d)", i),
				TestPath:    "/api/resources/drive/Home/Documents/" + benchDir + "/" + fname,
				Description: fmt.Sprintf("setup: big-dir file %d/%d", i+1, bigDirCount),
				Category:    "resources",
				Phase:       -2,
			})
		}
		routes = append(routes, RouteCase{
			Method:      "GET",
			Pattern:     "/api/resources/*path (bigdir list)",
			TestPath:    "/api/resources/drive/Home/Documents/" + benchDir + "/",
			Description: fmt.Sprintf("list big directory (%d files)", bigDirCount),
			Category:    "resources",
		})
	}

	// Assign current nginx proxy_read_timeout to each route
	for i := range routes {
		routes[i].CurrentTimeout = nginxTimeout(routes[i].ResolvePath())
	}

	// final cleanup: delete test directory (phase 99)
	routes = append(routes, RouteCase{
		Method:      "DELETE",
		Pattern:     "/api/resources/*path (cleanup)",
		TestPath:    "/api/resources/drive/Home/Documents/",
		Description: "cleanup: delete test directory",
		Category:    "resources",
		Phase:       99,
		Headers:     jsonCT,
		BodyFunc:    jsonBody(map[string]interface{}{"dirents": []string{"/" + benchDir}}),
	})

	// Also assign timeout to the final cleanup route
	routes[len(routes)-1].CurrentTimeout = nginxTimeout(routes[len(routes)-1].ResolvePath())

	return routes
}

// nginxTimeout returns the current nginx proxy_read_timeout for a given path,
// using longest-prefix matching to mirror nginx location block behavior.
func nginxTimeout(path string) string {
	type rule struct {
		prefix  string
		timeout string
	}
	// Order by longest prefix first for correct matching
	rules := []rule{
		{"/upload", "1800s"},
		{"/api/raw/AppData", "1800s"},
		{"/api/raw", "1800s"},
		{"/api/md5", "1800s"},
		{"/api/paste", "1800s"},
		{"/api/cache", "1800s"},
		{"/api/resources/AppData", "600s"},
		{"/api", "600s"},
		{"/callback", "600s"},
		{"/system", "600s"},
		{"/videos", "600s"},
		{"/seahub/", "600s"},
		{"/seafhttp/upload-aj", "600s"},
		{"/seafhttp/", "600s"},
		{"/provider", "60s"},
	}
	for _, r := range rules {
		if strings.HasPrefix(path, r.prefix) {
			return r.timeout
		}
	}
	return "60s"
}

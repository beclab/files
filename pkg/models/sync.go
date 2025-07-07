package models

import "time"

// SyncDir
type SyncDir struct {
	UserPerm   string         `json:"user_perm"`
	DirId      string         `json:"dir_id"`
	DirentList []*SyncDirInfo `json:"dirent_list"`
}

type SyncDirInfo struct {
	Type          string `json:"type"`
	Id            string `json:"id"`
	Name          string `json:"name"`
	Permission    string `json:"permission"`
	ParentDir     string `json:"parent_dir"`
	Size          int64  `json:"size"`
	FileSize      int64  `json:"fileSize"`
	NumTotalFiles int    `json:"numTotalFiles"`
	NumFiles      int    `json:"numFiles"`
	NumDirs       int    `json:"numDirs"`
	Path          string `json:"path"`
}

// SyncPathDetail
type SyncPathDetail struct {
	RepoId     string    `json:"repo_id"`
	Path       string    `json:"path"`
	Name       string    `json:"name"`
	Mtime      time.Time `json:"mtime"`
	Permission string    `json:"permission"`
}

// SyncFile
type SyncFile struct {
	FileId       string    `json:"file_id"`
	Path         string    `json:"path"`
	ParentDir    string    `json:"parent_dir"`
	FileName     string    `json:"filename"`
	FilePerm     string    `json:"rw"`
	LastModified int64     `json:"last_modified"`
	RawPath      string    `json:"raw_path"`
	Repo         *SyncRepo `json:"repo"`
}

type SyncRepo struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	Desc       string `json:"desc"`
	Version    int    `json:"version"`
	LastModify int64  `json:"last_modify"`
	Size       int64  `json:"size"`
	RepoId     string `json:"repo_id"`
	RepoName   string `json:"repo_name"`
}

// sync delete response
// {"success":true,"commit_id":"9cacb6c4be5af4906fb09b099aef36fe07c30763"}
type SyncDeleteResponse struct {
	Success  bool   `json:"success"`
	CommitID string `json:"commit_id"`
}

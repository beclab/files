namespace go api.search

struct GetDirectoriesResp {}

struct CheckDirectoryResp {
  1: string path (go.tag='json:"path"');
  2: i32 permission (go.tag='json:"permission"');
}

struct SearchDirectoryList {
  1: required string id (go.tag='db:"id" json:"id"');
  2: required string owner (go.tag='db:"owner" json:"owner"');
  3: required string fileType (go.tag='db:"file_type" json:"file_type"');
  4: required string extend (go.tag='db:"extend" json:"extend"');
  5: required string path (go.tag='db:"path" json:"path"');
  6: required string shareType (go.tag='db:"share_type" json:"share_type"');
  7: required string name (go.tag='db:"name" json:"name"');
  8: required i32 permission (go.tag='db:"permission" json:"permission"');
  9: required i32 memberPermission (go.tag='db:"member_permission" json:"member_permission"');
}

struct DirectoryInfo {
    1: string path (go.tag='json:"path"');
    2: string sharePath (go.tag='json:"sharePath"');
    3: string owner (go.tag='json:"owner"');
    4: i32 permission (go.tag='json:"permission"');
}


service SearchService {
    GetDirectoriesResp GetDirectories() (api.get="/api/search/get_directory/")
    CheckDirectoryResp CheckDirectory() (api.get="/api/search/check_directory/*path")
}
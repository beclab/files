namespace go api.search

struct GetDirectoriesResp {}

struct CheckDirectoryResp {}

struct DirectoryInfo {
    1: string path (go.tag='json:"path"');
    2: i32 permission (go.tag='json:"permission"');
}


service SearchService {
    GetDirectoriesResp GetDirectories() (api.get="/api/search/get_directory/")
    CheckDirectoryResp CheckDirectory() (api.get="/api/search/check_directory/*path")
}
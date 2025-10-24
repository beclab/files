namespace go api.search

struct SearchReq {
  1: string ReqId (api.body="reqId");
  2: string Keyword (api.body="keyword");
  3: string Type (api.body="type");
  4: string App (api.body="app");
  5: i64 Limit (api.body="limit");
  6: i64 Offset (api.body="offset");
}

struct SearchResp {}

struct GetAuthorityDirectoriesReq {
  1: string User (api.query="user");
}

struct GetAuthorityDirectoriesResp {}

struct CheckDirectoryAuthorityReq {
  1: string User (api.body="user");
  2: string Path (api.body="path");
}

struct CheckDirectoryAuthorityResp {
  1: bool read;
  2: bool write;
}


service SearchService {
  SearchResp Search(1: SearchReq request) (api.post="/api/search");

  GetAuthorityDirectoriesResp GetAuthorityDirectories(1: GetAuthorityDirectoriesReq request) (api.get="/api/authority");
  CheckDirectoryAuthorityResp CheckCheckDirectoryAuthority(1: CheckDirectoryAuthorityReq request) (api.post="/api/authority/*node");
}

namespace go api.search

struct SearchReq {
  1: string ReqId (api.body="reqId");
  2: string Keyword (api.body="keyword");
  3: string Type (api.body="type");
  4: string App (api.body="app");
  5: i64 Limit (api.body="limit");
  6: i64 Offset (api.body="offset");
}

struct SearchResp {

}

service SearchService {
  SearchResp Search(1: SearchReq request) (api.post="/api/share/search");
}

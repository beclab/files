namespace go api.tree

struct TreeResp {
}

service TreeService {
    TreeResp TreeMethod() (api.get="/api/tree/*path");
}

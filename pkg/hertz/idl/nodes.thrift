namespace go api.nodes

struct NodesNode {
    1: required string name;
    2: required bool master;
}

struct NodesData {
    1: required string currentNode;
    2: required list<NodesNode> nodes;
}

struct NodesResp {
    1: required i32 code;
    2: required NodesData data;
}

service NodesService {
    NodesResp NodesMethod() (api.get="/api/nodes");
}

namespace go api.md5

struct Md5Resp {
    1: required string md5;
}

service Md5Service {
    Md5Resp Md5Method() (api.get="/api/md5/*path");
}

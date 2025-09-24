namespace go api.users

struct UsersNode {
    1: required string name;
    2: required string role;
}

struct UsersData {
    1: required list<UsersNode> users;
}

struct UsersResp {
    1: required i32 code;
    2: required UsersData data;
}

service UsersService {
  UsersResp UsersMethod() (api.get="/api/users/");
}
namespace go callback

struct CallbackCreateReq {
    1: required string Name (api.body="name");
}

struct CallbackCreateResp {
}

struct CallbackDeleteReq {
    1: required string Name (api.body="name");
}

struct CallbackDeleteResp {
}

service CallbackService {
    CallbackCreateResp CallbackCreateMethod(1: CallbackCreateReq request) (api.post="/callback/create");
    CallbackDeleteResp CallbackDeleteMethod(1: CallbackDeleteReq request) (api.post="/callback/delete");
}
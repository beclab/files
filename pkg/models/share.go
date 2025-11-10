package models

type SambaShares struct {
	Id          string               `json:"id"`
	Owner       string               `json:"owner"`
	FileType    string               `json:"fileType"`
	Extend      string               `json:"extend"`
	Path        string               `json:"path"`
	ShareType   string               `json:"shareType"`
	Name        string               `json:"name"`
	ExpireIn    int64                `json:"expireIn"`
	ExpireTime  string               `json:"expireTime"`
	PublicShare bool                 `json:"publicShare"`
	Members     []*SambaShareMembers `json:"members"`
}

type SambaShareMembers struct {
	UserId     string `json:"userId"`
	UserName   string `json:"userName"`
	Password   string `json:"password"`
	Permission int32  `json:"permission"`
}

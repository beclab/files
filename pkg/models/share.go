package models

type SambaMemberUserView struct {
	PathId     string `db:"path_id"`
	UserId     string `db:"user_id"`
	UserName   string `db:"user_name"`
	Permission int32  `db:"permission"`
}

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
	Permission  int32                `json:"permission"`
	PublicShare bool                 `json:"publicShare"`
	Members     []*SambaShareMembers `json:"members"`
}

type SambaShareMembers struct {
	UserId     string `json:"userId"`
	UserName   string `json:"userName"`
	Password   string `json:"password"`
	Permission int32  `json:"permission"`
}

type InternalSharePath struct {
	Id                    string `json:"id"`
	Owner                 string `json:"owner"`
	FileType              string `json:"fileType"`
	Extend                string `json:"extend"`
	Path                  string `json:"path"`
	ShareType             string `json:"shareType"`
	Name                  string `json:"name"`
	ExpireIn              int64  `json:"expireIn"`
	ExpireTime            string `json:"expireTime"`
	Permission            int32  `json:"permission"`
	ShareMemberId         int64  `json:"share_member_id"`
	ShareMember           string `json:"share_member"`
	ShareMemberPermission int32  `json:"share_member_permission"`
}

type SharePathList struct {
	Id         string                       `json:"id"`
	Owner      string                       `json:"owner"`
	FileType   string                       `json:"file_type"`
	Extend     string                       `json:"extend"`
	Path       string                       `json:"path"`
	ShareType  string                       `json:"share_type"`
	Name       string                       `json:"name"`
	ExpireIn   int64                        `json:"expire_id"`
	ExpireTime string                       `json:"expire_time"`
	Permission int32                        `json:"permission"`
	CreateTime string                       `json:"create_time"`
	UpdateTime string                       `json:"update_time"`
	SharedByMe bool                         `json:"shared_by_me"`
	PublicSmb  *bool                        `json:"public_smb,omitempty"`
	Users      []*InternalSmbSharePathUsers `json:"users"`
	SmbLink    string                       `json:"smb_link,omitempty"`
}

type InternalSmbSharePath struct {
	Id         string                       `json:"id"`
	Owner      string                       `json:"owner"`
	FileType   string                       `json:"file_type"`
	Extend     string                       `json:"extend"`
	Path       string                       `json:"path"`
	ShareType  string                       `json:"share_type"`
	Name       string                       `json:"name"`
	ExpireIn   int64                        `json:"expire_id"`
	ExpireTime string                       `json:"expire_time"`
	Permission int32                        `json:"permission"`
	SharedByMe bool                         `json:"shared_by_me"`
	PublicSmb  *bool                        `json:"public_smb,omitempty"`
	Users      []*InternalSmbSharePathUsers `json:"users"`
	SmbLink    string                       `json:"smb_link,omitempty"`
}

type InternalSmbSharePathUsers struct {
	Id         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Permission int32  `json:"permission"`
}

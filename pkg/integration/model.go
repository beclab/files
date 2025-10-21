package integration

type Integrations struct {
	Tokens map[string]*IntegrationToken `json:"tokens"`
}

type IntegrationToken struct {
	Owner     string `json:"owner"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	ExpiresAt int64  `json:"expires_at"`
	Available bool   `json:"available"`
	Scope     string `json:"scope"`
	IdToken   string `json:"id_token"`
	ClientId  string `json:"client_id"`
}

type Header struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type accountResponse struct {
	Header
	Data *accountResponseData `json:"data,omitempty"`
}

type accountResponseData struct {
	Name     string                  `json:"name"`
	Type     string                  `json:"type"`
	RawData  *accountResponseRawData `json:"raw_data"`
	CloudUrl string                  `json:"cloudUrl"`
	ClientId string                  `json:"client_id"`
}

type accountResponseRawData struct {
	ExpiresAt    int64  `json:"expires_at"`
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
	Endpoint     string `json:"endpoint"`
	Bucket       string `json:"bucket"`
	UserId       string `json:"userid"`
	Available    bool   `json:"available"`
	CreateAt     int64  `json:"create_at"`
	Scope        string `json:"scope"`
	IdToken      string `json:"id_token"`
	ClientId     string `json:"client_id"`
}

type accountsResponse struct {
	Header
	Data []*accountsResponseData `json:"data,omitempty"`
}

type accountsResponseData struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	ExpiresAt int64  `json:"expires_at"`
	Available bool   `json:"available"`
	CreateAt  int64  `json:"create_at"`
}

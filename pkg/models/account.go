package models

type AccountResponse struct {
	Code int                   `json:"code"`
	Data []*AccounResponseItem `json:"data"`
}

type AccounResponseItem struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	ExpiresAt int64  `json:"expires_at"`
	Available bool   `json:"available"`
	CreateAt  int64  `json:"create_at"`
}

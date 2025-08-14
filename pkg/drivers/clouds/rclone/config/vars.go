package config

import (
	"files/pkg/common"
)

var (
	CreateConfigPath = "config/create"
	DeleteConfigPath = "config/delete"
	DumpConfigPath   = "config/dump"
)

type CreateConfigChanged struct {
	Create []*Config `json:"create,omitempty"`
	Update []*Config `json:"update,omitempty"`
	Delete []*Config `json:"delete,omitempty"`
}

type CreateConfig struct {
	Name       string      `json:"name"` // {owner}_{type}_{ACCESS_KEY}
	Type       string      `json:"type"`
	Parameters interface{} `json:"parameters,omitempty"`
}

type ConfigParameters struct {
	ConfigName      string `json:"config_name,omitempty"`
	Name            string `json:"name,omitempty"`
	Provider        string `json:"provider,omitempty"`
	AccessKeyId     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	Url             string `json:"url,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"` // s3
	Bucket          string `json:"bucket,omitempty"`   // s3

	Token        string `json:"token,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	ClientId     string `json:"client_id,omitempty"`

	AccessToken  string `json:"access_token,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
}

type Config struct {
	ConfigName      string `json:"config_name"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Provider        string `json:"provider"`
	AccessToken     string `json:"access_token"`
	SecretAccessKey string `json:"secret_access_key"`
	RefreshToken    string `json:"refresh_token"`
	ExpiresIn       int64  `json:"expires_in"`
	ExpiresAt       int64  `json:"expires_at"`
	Available       bool   `json:"available"`
	CreateAt        int64  `json:"create_at"`

	Token string `json:"token"`

	Url      string `json:"url"`
	Endpoint string `json:"endpoint"`
	Bucket   string `json:"bucket"`
	ClientId string `json:"client_id"`
}

func (c *Config) Equal(target *Config) bool {
	if c.Type == common.RcloneTypeS3 {
		if c.Url != target.Url || c.Endpoint != target.Endpoint || c.Bucket != target.Bucket || c.SecretAccessKey != target.SecretAccessKey {
			return false
		}
	} else if c.Type == common.RcloneTypeDropbox || c.Type == common.RcloneTypeDrive {
		if c.AccessToken != target.AccessToken || c.RefreshToken != target.RefreshToken || c.ExpiresAt != target.ExpiresAt {
			return false
		}
	}
	return true
}

type DropBoxToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	Expiry       string `json:"expiry"`
	ExpiresAt    int64  `json:"expires_at"`
}

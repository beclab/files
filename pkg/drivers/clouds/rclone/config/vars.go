package config

import "encoding/json"

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

// todo need to test
func (c *Config) Equal(target *Config) bool {
	if c.Type == "awss3" || c.Type == "tencent" {
		if c.Url != target.Url {
			return false
		}
		if c.Endpoint != target.Endpoint {
			return false
		}
		if c.Bucket != target.Bucket {
			return false
		}
		if c.SecretAccessKey != target.SecretAccessKey {
			return false
		}
	} else if c.Type == "dropbox" || c.Type == "drive" {
		var ct *DropBoxToken
		if err := json.Unmarshal([]byte(c.Token), &ct); err != nil {
			return false
		}

		if ct.AccessToken != target.AccessToken {
			return false
		}
		if c.RefreshToken != target.RefreshToken {
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
	ExpiresIn    int64  `json:"expires_at"`
}

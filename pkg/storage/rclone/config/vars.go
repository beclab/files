package config

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
	Name       string            `json:"name"` // {owner}_{type}_{ACCESS_KEY}
	Type       string            `json:"type"`
	Parameters *ConfigParameters `json:"parameters"`
}

type ConfigParameters struct {
	Name            string `json:"name,omitempty"`
	Provider        string `json:"provider,omitempty"`
	AccessKeyId     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`
	Url             string `json:"url,omitempty"`
	Endpoint        string `json:"endpoint,omitempty"` // s3
	Bucket          string `json:"bucket,omitempty"`   // s3
	// Region          string `json:"region"`   // s3
	// Prefix          string `json:"prefix"`   // s3
	Token        string `json:"token,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
	ClientId     string `json:"client_id,omitempty"`
}

type Config struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	Provider        string `json:"provider"`
	AccessKeyId     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	RefreshToken    string `json:"refresh_token"`
	Url             string `json:"url"`
	Endpoint        string `json:"endpoint"`
	Bucket          string `json:"bucket"`
	ClientId        string `json:"client_id"`
}

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
	} else if c.Type == "dropbox" {
		if c.SecretAccessKey != target.SecretAccessKey {
			return false
		}
		if c.ClientId != target.ClientId {
			return false
		}
		if c.RefreshToken != target.RefreshToken {
			return false
		}
	} else if c.Type == "google" { // todo
		return true
	}
	return true
}

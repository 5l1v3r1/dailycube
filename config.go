package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"strings"
	"time"
)

type Config struct {
	AdminHash   string `json:"admin_hash"`
	ClientID    string `json:"client_id"`
	Secret      string `json:"secret"`
	CallbackURI string `json:"callback_uri"`
	Timezone    string `json:"timezone"`
	Location    *time.Location
}

func LoadConfig(path string) (*Config, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var res Config
	if err := json.Unmarshal(contents, &res); err != nil {
		return nil, err
	}
	res.Location, err = time.LoadLocation(res.Timezone)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (c *Config) CheckPassword(pass string) bool {
	hash := sha256.Sum256([]byte(pass))
	readableHash := strings.ToLower(hex.EncodeToString(hash[:]))
	return readableHash == c.AdminHash
}

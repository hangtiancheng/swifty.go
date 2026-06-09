package config

import (
	"encoding/json"
	"log"
	"os"
)

type AppConfig struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type MongoConfig struct {
	URI      string `json:"uri"`
	Database string `json:"database"`
}

type CacheConfig struct {
	MaxBytes      int64  `json:"maxBytes"`
	Expiration    int    `json:"expiration"`
	DashboardAddr string `json:"dashboardAddr"`
}

type StaticConfig struct {
	AvatarPath string `json:"avatarPath"`
	FilePath   string `json:"filePath"`
}

type Config struct {
	App    AppConfig    `json:"app"`
	Mongo  MongoConfig  `json:"mongo"`
	Cache  CacheConfig  `json:"cache"`
	Static StaticConfig `json:"static"`
}

var conf *Config

func Load(path string) *Config {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read config: %v", err)
	}
	conf = &Config{}
	if err := json.Unmarshal(data, conf); err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}
	return conf
}

func Get() *Config {
	if conf == nil {
		log.Fatal("config not loaded")
	}
	return conf
}

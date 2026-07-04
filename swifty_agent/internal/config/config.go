// Package config provides configuration loading and types for the swifty_agent application.
// Configuration is loaded from a JSON file and provides settings for AI models,
// vector database connections, and application behavior.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds all application configuration values.
type Config struct {
	// ServerAddr is the address the HTTP server listens on (e.g., ":6872").
	ServerAddr string `json:"server_addr"`

	// ThinkChatModel configures the LLM used for deep reasoning tasks (planning, replanning).
	ThinkChatModel ChatModelConfig `json:"ds_think_chat_model"`

	// QuickChatModel configures the LLM used for fast chat responses and tool execution.
	QuickChatModel ChatModelConfig `json:"ds_quick_chat_model"`

	// EmbeddingModel configures the embedding model used for vectorization.
	EmbeddingModel EmbeddingConfig `json:"doubao_embedding_model"`

	// FileDir is the directory path for storing uploaded knowledge base files.
	FileDir string `json:"file_dir"`

	// MCP_URL is the Server-Sent Events endpoint for the MCP (Model Context Protocol) tool server.
	MCP_URL string `json:"mcp_url"`

	// Milvus configures the connection to the Milvus vector database.
	Milvus MilvusConfig `json:"milvus"`
}

// ChatModelConfig holds LLM connection settings for OpenAI-compatible API endpoints.
type ChatModelConfig struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

// EmbeddingConfig holds embedding model settings including dimension parameters.
type EmbeddingConfig struct {
	APIKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
}

// MilvusConfig holds Milvus vector database connection settings.
type MilvusConfig struct {
	Address string `json:"address"`
	DBName  string `json:"db_name"`
}

// Load reads and parses the configuration file at the given path.
// Missing fields are filled with sensible defaults.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	applyDefaults(cfg)
	return cfg, nil
}

// applyDefaults fills in zero-valued fields with sensible defaults.
func applyDefaults(cfg *Config) {
	if cfg.ServerAddr == "" {
		cfg.ServerAddr = ":6872"
	}
	if cfg.FileDir == "" {
		cfg.FileDir = "./docs"
	}
	if cfg.Milvus.Address == "" {
		cfg.Milvus.Address = "localhost:19530"
	}
	if cfg.Milvus.DBName == "" {
		cfg.Milvus.DBName = "agent"
	}
	if cfg.EmbeddingModel.Dimensions == 0 {
		cfg.EmbeddingModel.Dimensions = 2048
	}
}

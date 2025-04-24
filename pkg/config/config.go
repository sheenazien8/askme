package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Provider      string `yaml:"provider"`
	DefaultModel  string `yaml:"default_model"`
	OllamaURL     string `yaml:"ollama_url"`
	MistralAPIKey string `yaml:"mistral_api_key"`
	RoleSystem    string `yaml:"role_system"`
}

func ReadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %v", err)
	}

	configPath := filepath.Join(homeDir, ".config", "askme", "config.yaml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{}, nil
	}

	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	err = yaml.Unmarshal(configData, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}
	if config.OllamaURL == "" {
		config.OllamaURL = "http://localhost:11434/api/generate"
	}

	return &config, nil
}

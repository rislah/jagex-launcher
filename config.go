package main

import (
	"encoding/json"
	"os"
)

type Config struct {
	Accounts []Account `json:"accounts"`
	RuneLitePath string `json:"runelite_path"`
}

func loadConfig() (*Config, error) {
	configPath := getConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return &Config{Accounts: []Account{}}, nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	err = json.Unmarshal(data, &config)
	return &config, err
}

func saveConfig(config *Config) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	configPath := getConfigPath()
	return os.WriteFile(configPath, data, 0644)
}


package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	DbURL           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

func (conf *Config) SetUser(username string) error {
	existingConfig, err := Read()
	if err != nil {
		return err
	}
	existingConfig.CurrentUserName = username
	return write(existingConfig)
}

func Read() (Config, error) {
	filePath, err := getConfigFilePath()
	if err != nil {
		return Config{}, err
	}
	configFile, err := os.Open(filePath)
	if err != nil {
		return Config{}, err
	}
	defer configFile.Close()
	var data Config
	decoder := json.NewDecoder(configFile)
	if err := decoder.Decode(&data); err != nil {
		return Config{}, err
	}
	return data, err
}

func write(conf Config) error {
	jsonData, err := json.Marshal(conf)
	if err != nil {
		return err
	}
	configFilePath, err := getConfigFilePath()
	if err != nil {
		return err
	}
	return os.WriteFile(configFilePath, jsonData, 0644)
}

func getConfigFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configPath := homeDir + "/.gatorconfig.json"
	return configPath, nil
}

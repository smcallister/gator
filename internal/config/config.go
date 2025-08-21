package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Configuration struct {
	DBUrl string `json:"db_url"`
	CurrentUserName string `json: "current_user_name"`
}

const configFileName = ".gatorconfig.json"

func getConfigFilePath() (string, error) {
	// Get the home directory.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Get the full path to the file.
	return filepath.Join(homeDir, configFileName), nil
}

func Read() (*Configuration, error) {
	// Get the full path to the file.
	filePath, err := getConfigFilePath()
	if err != nil {
		return nil, err
	}
	
	// Read the config file.
    data, err := os.ReadFile(filePath)
	if err != nil {
        return nil, err
    }

	// Parse the config file.
	var config Configuration
	err = json.Unmarshal(data, &config)
	if err != nil {
        return nil, err
    }

	return &config, nil
}

func Write(config Configuration) error {
	// Get the full path to the file.
	filePath, err := getConfigFilePath()
	if err != nil {
		return err
	}

	// Marshall the configuration to JSON.
	jsonData, err := json.Marshal(config)
	if err != nil {
		return err
	}

	// Write the config to the file.
	file, err := os.Create(filePath)
    if err != nil {
        return err
    }

    defer file.Close()

    _, err = file.Write(jsonData)
    if err != nil {
        return err
    }

	return nil
}

func (c Configuration) SetUser(userName string) error {
	// Update the configuration.
	c.CurrentUserName = userName
	return Write(c)
}
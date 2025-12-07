package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"gaohoward.tools/k8s/resutil/pkg/logs"
	"go.uber.org/zap"
)

var logger *zap.Logger

func init() {
	logger, _ = logs.NewAppLogger("config")
}

const (
	DEFAULT_NAMESPACE = "default"
)

const APP_DIR = ".k8sutil"

type Config struct {
	CollectionRepoPaths []string `json:"collection_paths"`
}

func (c *Config) GetToolDir(toolName string) (string, error) {
	cfgDir, err := GetConfigDir()
	if err != nil {
		logger.Error("failed to get config dir", zap.Error(err))
		return "", err
	}
	toolRootDir := filepath.Join(cfgDir, "tools")
	if _, err := os.Stat(toolRootDir); os.IsNotExist(err) {
		err = os.MkdirAll(toolRootDir, 0755)
		if err != nil {
			logger.Error("failed to create tool root dir", zap.Error(err))
			return "", err
		}
	}
	toolDir := filepath.Join(toolRootDir, toolName)
	if _, err := os.Stat(toolDir); os.IsNotExist(err) {
		err = os.MkdirAll(toolDir, 0755)
		if err != nil {
			logger.Error("failed to create tool dir", zap.Error(err))
			return "", err
		}
	}
	return toolDir, nil
}

func GetConfigDir() (string, error) {
	// Check XDG_CONFIG_HOME or fallback to ~/.config
	configDir := os.Getenv("K8SUTIL_CONFIG_HOME")
	if configDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, APP_DIR)
	}

	// Create the directory if it doesn't exist
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			return "", err
		}
	}
	return configDir, nil
}

func GetResourceDetailsDir() (string, error) {
	cfgDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}

	resourceDetailsDir := filepath.Join(cfgDir, "resource-details")

	// Create the directory if it doesn't exist
	if _, err := os.Stat(resourceDetailsDir); os.IsNotExist(err) {
		err := os.MkdirAll(resourceDetailsDir, 0755)
		if err != nil {
			return "", err
		}
	}
	return resourceDetailsDir, nil
}

func GetConfig() (*Config, error) {
	cfgDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	config, err := LoadConfig(cfgDir)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func GetCollectionRepos() ([]string, error) {

	cfgDir, err := GetConfigDir()
	if err != nil {
		return nil, err
	}

	config, err := LoadConfig(cfgDir)
	if err != nil {
		return nil, err
	}

	localRepoPath := filepath.Join(cfgDir, "Internal")

	if _, err := os.Stat(localRepoPath); os.IsNotExist(err) {
		err = os.MkdirAll(localRepoPath, 0755)
		if err != nil {
			return nil, err
		}
	}

	repos := []string{
		localRepoPath,
	}

	if len(config.CollectionRepoPaths) > 0 {
		for _, r := range config.CollectionRepoPaths {
			repos = append(repos, r)
		}
	}

	return repos, nil
}

func LoadConfig(configDir string) (*Config, error) {
	var config Config

	configPath := filepath.Join(configDir, "config.json")
	file, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// no config.json
			return &config, nil
		}
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	return &config, err
}

// defines the configurable part of CollectionConfig
type CollectionConfigurable struct {
	Description string       `yaml:"description,omitempty"`
	Properties  []NamedValue `yaml:"properties,omitempty"`
}

type CollectionConfig struct {
	Id         string               `yaml:"id,omitempty"`
	Name       string               `yaml:"name,omitempty"`
	Attributes CollectionAttributes `yaml:"attributes,omitempty"`
	CollectionConfigurable
}

type CollectionAttributes struct {
	ResourceUrl string `yaml:"resourceUrl,omitempty"`
}

type NamedValue struct {
	Name  string `yaml:"name,omitempty"`
	Value string `yaml:"value,omitempty"`
}

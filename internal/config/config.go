package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

var (
	configErrors = []string{}
)

type Config struct {
	Core struct {
		ExternalURL      string
		ListeningAddress string
		LogPath          string
	} `yaml:"core"`

	Auth Auth `yaml:"auth"`

	Provider Provider `yaml:"provider"`
}

func defaultConfig() *Config {
	cfg := &Config{}

	cfg.Core.ExternalURL = "http://localhost:8080"
	cfg.Core.ListeningAddress = ":8080"
	cfg.Core.LogPath = "./test.log"

	return cfg
}

func GetConfig() (*Config, error) {
	cfg := defaultConfig()

	cfgFileName := "config/config.yaml"
	if envCfgFileName := os.Getenv("CONFIG_FILE"); envCfgFileName != "" {
		cfgFileName = envCfgFileName
	}

	if err := ReadConfigFromFile(cfg, cfgFileName); err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	return cfg, nil
}

func ReadConfigFromFile(conf *Config, path string) error {
	yamlFile, err := os.Open(path)
	if err != nil {
		return err
	}
	defer yamlFile.Close()

	decoder := yaml.NewDecoder(yamlFile)
	err = decoder.Decode(conf)
	if err != nil {
		return err
	}

	return nil
}

package config

import (
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type C struct{
    DBUrl            string `yaml:"dbUrl"`
    DefaultInterface string `yaml:"defaultInterface"`
}

const (
	configFolder = ".config"
	nls = "nls"
	config = "config.yaml"
)

func configPath() string{
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configFolder, nls, config)

}

func ReadConfig() (C, error){
	var config C
	yamlPath := configPath()
	data, err := os.ReadFile(yamlPath)
	if err != nil{
		slog.Error("There was an error in reading in the config", "error", err)
		return config, err
	}
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		slog.Error("The config could not be loaded please check config", "error", err)
		return config, err
	}
	return config, nil
}

func SaveConfig(dbUrl string, defaultInterface string) error {
    os.MkdirAll(filepath.Dir(configPath()), 0755)
    currentConfig, err := ReadConfig()
    if err != nil && !os.IsNotExist(err) {
        return err
    }
    currentConfig.DBUrl = dbUrl
    currentConfig.DefaultInterface = defaultInterface

    data, err := yaml.Marshal(currentConfig)
    if err != nil {
        return err
    }
    return os.WriteFile(configPath(), data, 0644)
}
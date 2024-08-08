package config

import (
	"cmd/energomer125-reader/internal/models"
	"fmt"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

const (
	localConfig   string = "c:\\Personal\\energomer125-reader\\configs\\config.yaml"
	releaseConfig string = "reporting-api.conf.yml"
)

var (
	GlobalConfig models.Configuration
)

func InitConfig() {
	configFiles := []string{localConfig, releaseConfig}
	var configName string
	for _, configFile := range configFiles {
		if _, err := os.Stat(configFile); err == nil {
			configName = configFile
			break
		}
	}

	viper.SetConfigFile(configName)
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println(err.Error())
	}

	err = viper.Unmarshal(&GlobalConfig)
	if err != nil {
		fmt.Println(err.Error())
	}

	// Настраиваем отслеживание изменений в конфигурационном файле
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed", e.Name)
		err := viper.Unmarshal(&GlobalConfig)
		if err != nil {
			fmt.Println(err.Error())
		}
	})
}

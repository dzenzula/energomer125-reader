package configurations

import (
	"fmt"
	l "main/logger"
	"main/models"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var (
	GlobalConfig models.Configuration
)

func InitConfig() {
	configFiles := []string{"configurations/config.yaml", "config.yaml", "energomer125-reader.conf.yml"}
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
		l.Fatal(err)
	}

	err = viper.Unmarshal(&GlobalConfig)
	if err != nil {
		l.Fatal(err)
	}

	// Настраиваем отслеживание изменений в конфигурационном файле
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed", e.Name)
		l.Info("Config file changed", e.Name)
		err := viper.Unmarshal(&GlobalConfig)
		if err != nil {
			l.Fatal(err)
		}
	})
}

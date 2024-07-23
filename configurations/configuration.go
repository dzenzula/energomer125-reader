package configurations

import (
	"fmt"
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

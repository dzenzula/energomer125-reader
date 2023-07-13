package configurations

import (
	"io/ioutil"
	l "main/logger"
	"main/models"
	"os"

	"gopkg.in/yaml.v2"
)

var (
	GlobalConfig models.Configuration
)

func InitConfig() {
	var config_files [2]string
	configName := "config.yaml"
	config_files[0] = "configurations/" + configName
	config_files[1] = configName
	for i := 0; i < len(config_files); i++ {
		if _, err := os.Stat(config_files[i]); err == nil {
			configName = config_files[i]
			break
		}
	}

	data, err := ioutil.ReadFile(configName)
	if err != nil {
		l.Fatal(err)
	}

	err = yaml.Unmarshal(data, &GlobalConfig)
	if err != nil {
		l.Fatal(err)
	}
}

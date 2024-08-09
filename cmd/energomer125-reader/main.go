package main

import (
	c "cmd/energomer125-reader/internal/config"
	"cmd/energomer125-reader/internal/energomer"
	"cmd/energomer125-reader/internal/models"
	st "cmd/energomer125-reader/internal/storage_time"
	"time"

	log "krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging"
)

func main() {
	initializeApplication()
	archiveChan := make(chan models.Command, 10)
	go energomer.ProcessArchives(archiveChan)

	for {
		wait()
		energomer.ProcessCommands(archiveChan)
		log.Info("Ended transfer data")
	}
}

func initializeApplication() {
	c.InitConfig()
	log.LogInit(c.GlobalConfig.Log_Level)
	log.Info("Service started!")

	st.InitTimeManager()
}

func wait() {
	duration := time.Until(time.Now().Truncate(c.GlobalConfig.Timer).Add(c.GlobalConfig.Timer))
	time.Sleep(duration)
	log.Info("Started transfer data")
}

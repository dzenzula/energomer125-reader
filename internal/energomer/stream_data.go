package energomer

import (
	"cmd/energomer125-reader/internal/config"
	"cmd/energomer125-reader/internal/models"
	"cmd/energomer125-reader/internal/network"
	rh "cmd/energomer125-reader/internal/response_handler"
	st "cmd/energomer125-reader/internal/storage_time"
	"fmt"
	"time"

	log "krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging"
)

func ProcessCommands(archiveChan chan<- models.Command) {
	for _, energomer := range config.GlobalConfig.Commands {
		if err := getStreamData(energomer); err == nil {
			if inProcess, ok := ArchiveInProcess.Load(energomer.Current_Data); !ok || !inProcess.(bool) {
				archiveChan <- energomer
			}
			st.GlobalTimeManager.UpdateSuccessfulRetrieval(energomer.Current_Data)
		}
	}
}

func getStreamData(energomer models.Command) error {
	for retriesLeft := config.GlobalConfig.Max_Read_Retries; retriesLeft > 0; retriesLeft-- {
		cm = network.NewConnectionManager()
		err := cm.SetupConnection(energomer)
		if err != nil {
			log.Error(err.Error())
			continue
		}
		defer cm.CloseConnection()

		if err := cm.SendCommand(energomer.Current_Data); err != nil {
			log.Error(fmt.Sprintf("Send command failed: %s", err.Error()))
			return err
		}

		response, err := cm.ReadResponse(energomer.Last_Hour_Archive)
		if err != nil {
			log.Error(err.Error())
			continue
		}

		err = rh.ProcessResponse(response, energomer)
		if err != nil {
			log.Error(err.Error())
			continue
		}

		return nil
	}
	handleMaxRetriesReached(energomer)
	return fmt.Errorf("reached max retries")
}

func handleMaxRetriesReached(energomer models.Command) {
	st.GlobalTimeManager.SetCurrentSuccessfulRetrieval(energomer, time.Date(1, 0, 0, 0, 0, 0, 0, time.Local))
	log.Error("Reached maximum retries, unable to retrieve valid data.")
}

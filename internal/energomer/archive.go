package energomer

import (
	c "cmd/energomer125-reader/internal/config"
	"cmd/energomer125-reader/internal/models"
	"cmd/energomer125-reader/internal/network"
	rh "cmd/energomer125-reader/internal/response_handler"
	st "cmd/energomer125-reader/internal/storage_time"
	"fmt"
	"time"

	log "krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging"
)

var cm *network.ConnectionManager

func ProcessArchives(archiveChan <-chan models.Command) {
	for energometer := range archiveChan {
		retrieveMissingData(energometer)
	}
}

func retrieveMissingData(energomer models.Command) {
	for retriesLeft := c.GlobalConfig.Max_Read_Retries; retriesLeft > 0; retriesLeft-- {
		st.GlobalTimeManager.LoadTimeFromFile()
		current := st.GlobalTimeManager.GetCurrentSuccessfulRetrieval()
		if current[energomer.Current_Data].Year() != time.Now().Year() {
			return
		}

		diff := st.GlobalTimeManager.CalculateTimeDifference(energomer)
		if diff <= 1 {
			return
		}

		cm = network.NewConnectionManager()
		err := cm.SetupConnection(energomer)
		if err != nil {
			log.Error(err.Error())
			continue
		}
		defer cm.CloseConnection()

		err = cm.SendCommand(energomer.Last_Hour_Archive)
		if err != nil {
			log.Error(err.Error())
			continue
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

		retrieveArchiveData(energomer, diff, response)

		return
	}
}

func retrieveArchiveData(energomer models.Command, diff int, response []byte) {
	for i := 0; i < diff; i++ {
		if !rh.IsValidDateTime(response) {
			i--
			continue
		}

		if err := cm.SendCommand(energomer.Backwards_Archive); err != nil {
			i--
			log.Error(fmt.Sprintf("Send command failed: %s", err.Error()))
			continue
		}

		response, err := cm.ReadResponse(energomer.Backwards_Archive)
		if err != nil {
			i--
			log.Error(fmt.Sprintf("Read response failed: %s", err.Error()))
			continue
		}

		err = rh.ProcessResponse(response, energomer)
		if err != nil {
			i--
			log.Error(err.Error())
			continue
		}
	}
}

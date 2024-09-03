package energomer

import (
	"cmd/energomer125-reader/internal/models"
	"cmd/energomer125-reader/internal/network"
	rh "cmd/energomer125-reader/internal/response_handler"
	st "cmd/energomer125-reader/internal/storage_time"
	"fmt"
	"sync"
	"time"

	log "krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging"
)

var (
	cm               *network.ConnectionManager
	ArchiveInProcess sync.Map
)

func ProcessArchives(archiveChan <-chan models.Command) {
	var wg sync.WaitGroup

	for energometer := range archiveChan {
		wg.Add(1)
		ArchiveInProcess.Store(energometer.Current_Data, true)
		go func(e models.Command) {
			defer wg.Done()
			retrieveMissingData(e)
			ArchiveInProcess.Store(e.Current_Data, false)
		}(energometer)
	}

	wg.Wait()
}

func retrieveMissingData(energomer models.Command) {
	st.GlobalTimeManager.LoadTimeFromFile()
	for {
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

		retrieveArchiveData(energomer, diff, response, cm)

		cm.CloseConnection()
		return
	}
}

func retrieveArchiveData(energomer models.Command, diff int, response []byte, cm *network.ConnectionManager) {
	for i := 0; i < diff; i++ {
		err := cm.SetupConnection(energomer)
		if err != nil {
			log.Error(err.Error())
			time.Sleep(time.Second * 10)
			cm = network.NewConnectionManager()
			i--
			continue
		}

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
			time.Sleep(time.Second * 10)
			continue
		}
	}
}

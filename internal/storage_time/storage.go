package storage_time

import (
	"cmd/energomer125-reader/internal/models"
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	log "krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging"
)

type TimeStorage struct {
	LastSuccessfulRetrieval map[string]time.Time `yaml:"lastSuccessfulRetrieval"`
}

type TimeManager struct {
	lastSuccessfulRetrieval    map[string]time.Time
	currentSuccessfulRetrieval map[string]time.Time
}

const timeStorageFile = "time_storage.yaml"

var (
	GlobalTimeManager *TimeManager
	retrievalMutex    sync.Mutex
)

func InitTimeManager() {
	GlobalTimeManager = &TimeManager{
		lastSuccessfulRetrieval:    make(map[string]time.Time),
		currentSuccessfulRetrieval: make(map[string]time.Time),
	}
	GlobalTimeManager.LoadTimeFromFile()
}

func (tm *TimeManager) SaveTimeToFile() {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	storage := TimeStorage{
		LastSuccessfulRetrieval: tm.currentSuccessfulRetrieval,
	}

	data, err := yaml.Marshal(&storage)
	if err != nil {
		log.Error(fmt.Sprintf("Error marshalling time data: %s", err))
		return
	}

	err = os.WriteFile(timeStorageFile, data, 0644)
	if err != nil {
		log.Error(fmt.Sprintf("Error writing time data to file: %s", err))
		return
	}

	log.Debug("Time data successfully saved to file.")
}

func (tm *TimeManager) LoadTimeFromFile() {
	data, err := os.ReadFile(timeStorageFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info("Time storage file not found. Creating a new one.")
			tm.SaveTimeToFile()
			return
		}
		log.Error(fmt.Sprintf("Error reading time data from file: %s", err))
		return
	}

	var storage TimeStorage
	err = yaml.Unmarshal(data, &storage)
	if err != nil {
		log.Error(fmt.Sprintf("Error unmarshalling time data: %s", err))
		return
	}

	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	tm.lastSuccessfulRetrieval = storage.LastSuccessfulRetrieval
}

func (tm *TimeManager) GetLastSuccessfulRetrieval() map[string]time.Time {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	// Возвращаем копию карты
	result := make(map[string]time.Time)
	for k, v := range tm.lastSuccessfulRetrieval {
		result[k] = v
	}
	return result
}

func (tm *TimeManager) GetCurrentSuccessfulRetrieval() map[string]time.Time {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	// Возвращаем копию карты
	result := make(map[string]time.Time)
	for k, v := range tm.currentSuccessfulRetrieval {
		result[k] = v
	}
	return result
}

func (tm *TimeManager) SetCurrentSuccessfulRetrieval(energomer models.Command, setTime time.Time) {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	tm.currentSuccessfulRetrieval[energomer.Current_Data] = setTime
}

func (tm *TimeManager) CalculateTimeDifference(energomer models.Command) int {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	lastRetrieval, exists := tm.lastSuccessfulRetrieval[energomer.Current_Data]
	if !exists {
		lastRetrieval = time.Now().Add(-1 * time.Hour)
	}
	currentRetrieval := time.Now()
	lastRetrieval = lastRetrieval.Truncate(time.Hour)
	currentRetrieval = currentRetrieval.Truncate(time.Hour)

	diff := int(currentRetrieval.Sub(lastRetrieval).Hours())
	log.Debug(fmt.Sprintf("Time difference: %d", diff))
	return min(diff, 24)
}

func (tm *TimeManager) UpdateSuccessfulRetrieval(command string) {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	if tm.lastSuccessfulRetrieval == nil {
		tm.lastSuccessfulRetrieval = make(map[string]time.Time)
	}

	if tm.currentSuccessfulRetrieval[command].Year() == time.Now().Year() {
		tm.lastSuccessfulRetrieval[command] = tm.currentSuccessfulRetrieval[command]
	}
	tm.currentSuccessfulRetrieval[command] = time.Now()

	log.Debug(fmt.Sprintf("Last successful retrieval: %s", tm.lastSuccessfulRetrieval[command].String()))
	log.Debug(fmt.Sprintf("Current successful retrieval: %s", tm.currentSuccessfulRetrieval[command].String()))

	go tm.SaveTimeToFile()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

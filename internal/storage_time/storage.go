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

var (
	lastSuccessfulRetrieval    map[string]time.Time = make(map[string]time.Time)
	currentSuccessfulRetrieval map[string]time.Time = make(map[string]time.Time)
	retrievalMutex             sync.Mutex
)

const timeStorageFile = "time_storage.yaml"

func SaveTimeToFile() {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	storage := TimeStorage{
		LastSuccessfulRetrieval: currentSuccessfulRetrieval,
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

func LoadTimeFromFile() {
	data, err := os.ReadFile(timeStorageFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info("Time storage file not found. Creating a new one.")
			SaveTimeToFile()
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

	lastSuccessfulRetrieval = storage.LastSuccessfulRetrieval
}

func GetLastSuccessfulRetrieval() map[string]time.Time {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	return lastSuccessfulRetrieval
}

func GetCurrentSuccessfulRetrieval() map[string]time.Time {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	return currentSuccessfulRetrieval
}

func SetCurrentSuccessfulRetrieval(energomer models.Command, setTime time.Time) {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	currentSuccessfulRetrieval[energomer.Current_Data] = setTime
}

func CalculateTimeDifference(energomer models.Command) int {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	lastRetrieval, exists := lastSuccessfulRetrieval[energomer.Current_Data]
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

func UpdateSuccessfulRetrieval(command string) {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	if currentSuccessfulRetrieval[command].Year() == time.Now().Year() {
		lastSuccessfulRetrieval[command] = currentSuccessfulRetrieval[command]
	}
	currentSuccessfulRetrieval[command] = time.Now()

	log.Debug(fmt.Sprintf("Last successful retrieval: %s", lastSuccessfulRetrieval[command].String()))
	log.Debug(fmt.Sprintf("Current successful retrieval: %s", currentSuccessfulRetrieval[command].String()))

	go SaveTimeToFile()
}
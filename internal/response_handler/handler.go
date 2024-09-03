package response_handler

import (
	"cmd/energomer125-reader/internal/database"
	"cmd/energomer125-reader/internal/models"
	"encoding/binary"
	"fmt"
	"math"
	"time"

	log "krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging"
)

const responseLength int = 132

func ProcessResponse(response []byte, energomer models.Command) error {
	if !isValidResponse(response) {
		return fmt.Errorf("response is not valid")
	}

	if isErrorResponse(response) {
		return fmt.Errorf("response has error flag")
	}

	date, dateTime := processDate(response)
	if date == "" {
		return fmt.Errorf("date is empty")
	}

	intSlice := convertResponseToIntSlice(response)
	log.Debug(fmt.Sprintf("Received data from the energometer: %d", intSlice))

	q1 := calculateQ1(response, dateTime)
	if !isValidQ1(q1) {
		log.Debug(fmt.Sprintf("Real q1: %f", q1))
		q1 = 0
	}

	err := database.InsertData(q1, energomer, date)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Date: %s, Command: %s Q1: %f", date, energomer.Current_Data, q1))
	return nil
}

func isValidResponse(response []byte) bool {
	if len(response) < responseLength {
		log.Error("Response is short")
		return false
	}
	return true
}

func isErrorResponse(response []byte) bool {
	if response[9] == 1 {
		log.Error("This response has error flag!")
		intSlice := convertResponseToIntSlice(response)
		log.Error(fmt.Sprintf("Received data from the energometer: %d", intSlice))
		return true
	}
	return false
}

func processDate(response []byte) (string, time.Time) {
	date := bytesToDateTime(response[0:6])
	if !checkDate(date) {
		log.Error("Date is wrong! Trying to get the right date...")
		return "", time.Time{}
	}

	layout := "2006-01-02 15:04:05"
	dateTime, _ := time.Parse(layout, date)
	if dateTime.Second() == 0 && dateTime.Minute() == 0 {
		dateTime = dateTime.Add(-1 * time.Hour)
		date = dateTime.Format(layout)
	}

	return date, dateTime
}

func convertResponseToIntSlice(response []byte) []int {
	intSlice := make([]int, len(response))
	for i, b := range response {
		intSlice[i] = int(b)
	}
	return intSlice
}

func calculateQ1(response []byte, dateTime time.Time) float32 {
	if len(response) == responseLength || (dateTime.Second() == 0 && dateTime.Minute() == 0) {
		return bytesToFloat32(response[14:18])
	}
	return bytesToFloat32(response[24:28])
}

func isValidQ1(q1 float32) bool {
	return q1 <= 10000 && q1 >= 0
}

func bytesToFloat32(data []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(data))
}

func bytesToDateTime(bytes []byte) string {
	year := int(bytes[5]) + 2000
	month := int(bytes[4])
	day := int(bytes[3])
	hour := int(bytes[2])
	minute := int(bytes[1])
	second := int(bytes[0])

	dateTime := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
	return dateTime.Format("2006-01-02 15:04:05")
}

func checkDate(date string) bool {
	if date == "" {
		log.Error("Date is empty")
		return false
	}

	dateTime, err := time.Parse("2006-01-02", date[:10])
	if err != nil {
		log.Error(fmt.Sprintf("Error when converting a string to a date: %s", err))
		return false
	}

	dateTime = dateTime.UTC()
	currentDate := time.Now().UTC().Truncate(24 * time.Hour)
	yesterdayDate := currentDate.Add(-48 * time.Hour)
	tomorrowDate := currentDate.Add(48 * time.Hour)

	return dateTime.After(yesterdayDate) && dateTime.Before(tomorrowDate)
}

func IsValidDateTime(response []byte) bool {
	date := bytesToDateTime(response[0:6])
	layout := "2006-01-02 15:04:05"
	dateTime, err := time.Parse(layout, date)
	if err != nil {
		log.Error(fmt.Sprintf("Error when converting a string to a date: %s", err))
		return false
	}

	return dateTime.Minute() == 0 && dateTime.Second() == 0
}

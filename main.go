package main

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	c "main/configurations"
	"main/models"
	"math"
	"net"
	"os"
	"sync"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	log "krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging"
)

type TimeStorage struct {
	LastSuccessfulRetrieval map[string]time.Time `json:"lastSuccessfulRetrieval"`
}

var (
	lastSuccessfulRetrieval    map[string]time.Time
	currentSuccessfulRetrieval map[string]time.Time
	retrievalMutex             sync.Mutex
)

const timeStorageFile = "time_storage.json"

func main() {
	c.InitConfig()
	log.LogInit(c.GlobalConfig.Log_Level)
	log.Info("Service started!")

	lastSuccessfulRetrieval = make(map[string]time.Time)
	currentSuccessfulRetrieval = make(map[string]time.Time)

	loadTimeFromFile()

	archiveChan := make(chan models.Command, 10)
	go processArchives(archiveChan)

	for {
		wait()

		for _, i := range c.GlobalConfig.Commands {
			getData(i)
			archiveChan <- i
		}
		log.Info("Ended transfer data")
	}
}

func getData(energometer models.Command) {
	for retriesLeft := c.GlobalConfig.Max_Read_Retries; retriesLeft > 0; retriesLeft-- {
		conn, err := createConnection(energometer)
		if err != nil {
			log.Error(fmt.Sprintf("Connection setup failed: %s", err.Error()))
			continue
		}
		defer conn.Close()

		if err := sendCommand(conn, energometer.Current_Data); err != nil {
			log.Error(fmt.Sprintf("Send command failed: %s", err.Error()))
			continue
		}

		response, err := readResponse(conn)
		if err != nil {
			log.Error(fmt.Sprintf("Read response failed: %s", err.Error()))
			continue
		}

		if validateResponse(response) {
			processEnergometerResponse(response, energometer, conn)
			updateSuccessfulRetrieval(energometer.Current_Data)
			return
		} else {
			log.Error(fmt.Sprintf("Received wrong data from the energometer: %v", response))
		}

	}
	currentSuccessfulRetrieval[energometer.Current_Data] = time.Date(1, 0, 0, 0, 0, 0, 0, time.Local)
	log.Error("Reached maximum retries, unable to retrieve valid data.")
}

func createConnection(energometer models.Command) (*net.TCPConn, error) {
	tcpServer, err := net.ResolveTCPAddr(c.GlobalConfig.Connection.Type, c.GlobalConfig.Connection.Host+":"+energometer.Port)
	if err != nil {
		return nil, fmt.Errorf("resolveTCPAddr failed: %w", err)
	}

	conn, err := net.DialTCP(c.GlobalConfig.Connection.Type, nil, tcpServer)
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}
	return conn, nil
}

func sendCommand(conn *net.TCPConn, command string) error {
	bytecommand := []byte(command)

	_, err := conn.Write(bytecommand)
	if err != nil {
		conn.Close()
		return fmt.Errorf("write failed: %w", err)
	}

	log.Info(fmt.Sprintf("Command: %s sent successfully!", command))
	return nil
}

func readResponse(conn *net.TCPConn) ([]byte, error) {
	response := make([]byte, 0)
	buffer := make([]byte, 4096)

	conn.SetReadDeadline(time.Now().Add(c.GlobalConfig.Timeout))

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			if err == io.EOF {
				log.Error("Connection closed by the server.")
				break
			}
			return nil, fmt.Errorf("read failed: %w", err)
		}

		response = append(response, buffer[:n]...)
		log.Debug(fmt.Sprintf("Bytes of information received: %d", n))

		if len(response) >= 132 {
			break
		}
	}
	return response, nil
}

func validateResponse(response []byte) bool {
	return len(response) >= 132
}

func processEnergometerResponse(response []byte, energometer models.Command, conn *net.TCPConn) {
	date := bytesToDateTime(response[0:6])
	if !checkDate(date) {
		log.Error("Date is wrong! Trying to get the right date...")
		if !isConnectionClosed(conn) {
			conn.Close()
		}
		return
	}

	layout := "2006-01-02 15:04:05"
	dateTime, _ := time.Parse(layout, date)
	sec := dateTime.Second()
	min := dateTime.Minute()
	if sec == 0 && min == 0 {
		dateTime = dateTime.Add(-1 * time.Hour)
		date = dateTime.Format("2006-01-02 15:04:05")
	}

	intSlice := make([]int, len(response))
	for i, b := range response {
		intSlice[i] = int(b)
	}

	log.Debug(fmt.Sprintf("Received data from the energometer: %d", intSlice))

	var q1 float32

	if len(response) == 132 {
		q1 = bytesToFloat32(response[14:18])
	} else {
		q1 = bytesToFloat32(response[24:28])
	}

	if q1 < 0 {
		q1 = 0
	} else if q1 > 10000 {
		if !isConnectionClosed(conn) {
			conn.Close()
		}
		return
	}

	insertData(q1, energometer, date)

	log.Info(fmt.Sprintf("%s Q1: %f", energometer.Current_Data, q1))
}

func insertData(v1 float32, energometr models.Command, date string) {
	db := ConnectMs()
	q := c.GlobalConfig.Query_Insert

	_, err := db.Exec(q, energometr.Name, energometr.Id_Measuring, v1, date, 192, nil)
	if err != nil {
		log.Error(fmt.Sprintf("Error during SQL query execution: %s", err.Error()))
	}

	defer db.Close()
}

func ConnectMs() *sql.DB {
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;database=%s", c.GlobalConfig.MSSQL.Server, c.GlobalConfig.MSSQL.User_Id, c.GlobalConfig.MSSQL.Password, c.GlobalConfig.MSSQL.Database)
	conn, conErr := sql.Open("mssql", connString)
	if conErr != nil {
		log.Error(fmt.Sprintf("Error opening database connection: %s", conErr.Error()))
	}

	pingErr := conn.Ping()
	if pingErr != nil {
		log.Error(pingErr.Error())
		time.Sleep(10 * time.Second)
		log.Error("Trying to reconnect to the database...")
		ConnectMs()
	}

	return conn
}

func bytesToFloat32(data []byte) float32 {
	bin := binary.LittleEndian.Uint32(data)
	res := math.Float32frombits(bin)
	return res
}

func bytesToDateTime(bytes []byte) string {
	year := int(bytes[5]) + 2000
	month := int(bytes[4])
	day := int(bytes[3])
	hour := int(bytes[2])
	minute := int(bytes[1])
	second := int(bytes[0])

	dateTime := time.Date(year, time.Month(month), day, hour, minute, second, 0, time.UTC)
	formattedDateTime := dateTime.Format("2006-01-02 15:04:05")

	return formattedDateTime
}

func wait() {
	duration := time.Until(time.Now().Truncate(c.GlobalConfig.Timer).Add(c.GlobalConfig.Timer))
	time.Sleep(duration)
	log.Info("Started transfer data")
}

func isConnectionClosed(conn net.Conn) bool {
	tempBuf := make([]byte, 1)
	conn.SetReadDeadline(time.Now())
	_, err := conn.Read(tempBuf)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return true
		}
	}
	return false
}

func checkDate(date string) bool {
	layout := "2006-01-02"
	dateTime, err := time.Parse(layout, date[:10])
	if err != nil {
		log.Error(fmt.Sprintf("Error when converting a string to a date: %s", err))
		return false
	}

	dateTime = dateTime.UTC()

	currentDate := time.Now().UTC().Truncate(24 * time.Hour)
	yesterdayDate := currentDate.Add(-48 * time.Hour)
	tomorrowDate := currentDate.Add(48 * time.Hour)

	if dateTime.After(yesterdayDate) && dateTime.Before(tomorrowDate) {
		return true
	}

	return false
}

func processArchives(archiveChan <-chan models.Command) {
	for energometer := range archiveChan {
		retrieveMissingData(energometer)
	}
}

func retrieveMissingData(energometer models.Command) {
	if currentSuccessfulRetrieval[energometer.Current_Data].Year() == -1 {
		return
	}

	retrievalMutex.Lock()
	lastRetrieval, exists := lastSuccessfulRetrieval[energometer.Current_Data]
	currentRetrieval := currentSuccessfulRetrieval[energometer.Current_Data]
	retrievalMutex.Unlock()

	if !exists {
		lastRetrieval = time.Now().Add(-1 * time.Hour) // Default to 1 hour ago if no record exists
	}

	lastRetrieval = lastRetrieval.Truncate(time.Hour)
	currentRetrieval = currentRetrieval.Truncate(time.Hour)

	diff := int(currentRetrieval.Sub(lastRetrieval).Hours())

	//fmt.Println("Differance: ", diff)

	if diff == 0 || diff == 1 {
		return
	} else if diff > 12 {
		diff = 12
	}

	conn, err := createConnection(energometer)
	if err != nil {
		log.Error(fmt.Sprintf("Connection setup failed: %s", err.Error()))
		return
	}
	defer conn.Close()

	if err := sendCommand(conn, energometer.Last_Hour_Archive); err != nil {
		log.Error(fmt.Sprintf("Send command failed: %s", err.Error()))
		return
	}

	response, err := readResponse(conn)
	if err != nil {
		log.Error(fmt.Sprintf("Read response failed: %s", err.Error()))
		return
	}

	processEnergometerResponse(response, energometer, conn)

	for i := 0; i < diff; {
		date := bytesToDateTime(response[0:6])
		layout := "2006-01-02 15:04:05"
		dateTime, err := time.Parse(layout, date)
		if err != nil {
			log.Error(fmt.Sprintf("Error when converting a string to a date: %s", err))
			return
		}

		sec := dateTime.Second()
		minute := dateTime.Minute()

		if minute != 0 && sec != 0 {
			continue
		}

		if err := sendCommand(conn, energometer.Backwards_Archive); err != nil {
			log.Error(fmt.Sprintf("Send command failed: %s", err.Error()))
			return
		}

		response, err := readResponse(conn)
		if err != nil {
			log.Error(fmt.Sprintf("Read response failed: %s", err.Error()))
			return
		}

		processEnergometerResponse(response, energometer, conn)

		i++
	}
}

func updateSuccessfulRetrieval(command string) {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	y := currentSuccessfulRetrieval[command].Year()
	//fmt.Println(y)

	if y != 1 {
		lastSuccessfulRetrieval[command] = currentSuccessfulRetrieval[command]
	}
	currentSuccessfulRetrieval[command] = time.Now()

	log.Debug(fmt.Sprintf("Last successful retrieval: %s", lastSuccessfulRetrieval[command].String()))
	log.Debug(fmt.Sprintf("Current successful retrieval: %s", currentSuccessfulRetrieval[command].String()))

	go saveTimeToFile()
}

func saveTimeToFile() {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	storage := TimeStorage{
		LastSuccessfulRetrieval: lastSuccessfulRetrieval,
	}

	data, err := json.MarshalIndent(storage, "", "  ")
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

func loadTimeFromFile() {
	data, err := os.ReadFile(timeStorageFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Info("Time storage file not found. Creating a new one.")
			saveTimeToFile() // Создаем новый файл
			return
		}
		log.Error(fmt.Sprintf("Error reading time data from file: %s", err))
		return
	}

	var storage TimeStorage
	err = json.Unmarshal(data, &storage)
	if err != nil {
		log.Error(fmt.Sprintf("Error unmarshalling time data: %s", err))
		return
	}

	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	lastSuccessfulRetrieval = storage.LastSuccessfulRetrieval
}

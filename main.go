package main

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"sync"
	"time"

	c "main/configurations"
	"main/models"

	_ "github.com/denisenkom/go-mssqldb"
	log "krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging"
)

// TimeStorage represents the structure for storing last successful retrieval times
type TimeStorage struct {
	LastSuccessfulRetrieval map[string]time.Time `json:"lastSuccessfulRetrieval"`
}

var (
	lastSuccessfulRetrieval    map[string]time.Time
	currentSuccessfulRetrieval map[string]time.Time
	retrievalMutex             sync.Mutex
)

const (
	timeStorageFile = "time_storage.json"
	responseLength  = 132
)

func main() {
	initializeApplication()
	archiveChan := make(chan models.Command, 10)
	go processArchives(archiveChan)

	for {
		wait()
		processCommands(archiveChan)
		log.Info("Ended transfer data")
	}
}

func initializeApplication() {
	c.InitConfig()
	log.LogInit(c.GlobalConfig.Log_Level)
	log.Info("Service started!")

	lastSuccessfulRetrieval = make(map[string]time.Time)
	currentSuccessfulRetrieval = make(map[string]time.Time)

	loadTimeFromFile()
}

func processCommands(archiveChan chan<- models.Command) {
	for _, command := range c.GlobalConfig.Commands {
		getData(command)
		archiveChan <- command
	}
}

func getData(energometer models.Command) {
	for retriesLeft := c.GlobalConfig.Max_Read_Retries; retriesLeft > 0; retriesLeft-- {
		if processEnergometerData(energometer) {
			return
		}
	}
	handleMaxRetriesReached(energometer)
}

func processEnergometerData(energometer models.Command) bool {
	conn, err := createConnection(energometer)
	if err != nil {
		log.Error(fmt.Sprintf("Connection setup failed: %s", err.Error()))
		return false
	}
	defer conn.Close()

	if err := sendCommand(conn, energometer.Current_Data); err != nil {
		log.Error(fmt.Sprintf("Send command failed: %s", err.Error()))
		return false
	}

	response, err := readResponse(conn)
	if err != nil {
		log.Error(fmt.Sprintf("Read response failed: %s", err.Error()))
		return false
	}

	if validateResponse(response) {
		processEnergometerResponse(response, energometer, conn)
		updateSuccessfulRetrieval(energometer.Current_Data)
		return true
	}

	log.Error(fmt.Sprintf("Received wrong data from the energometer: %v", response))
	return false
}

func handleMaxRetriesReached(energometer models.Command) {
	currentSuccessfulRetrieval[energometer.Current_Data] = time.Date(1, 0, 0, 0, 0, 0, 0, time.Local)
	log.Error("Reached maximum retries, unable to retrieve valid data.")
}

func createConnection(energometer models.Command) (*net.TCPConn, error) {
	tcpServer, err := net.ResolveTCPAddr(c.GlobalConfig.Connection.Type, fmt.Sprintf("%s:%s", c.GlobalConfig.Connection.Host, energometer.Port))
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
	_, err := conn.Write([]byte(command))
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
			if handleConnectionError(err) {
				break
			}
			return nil, fmt.Errorf("read failed: %w", err)
		}

		response = append(response, buffer[:n]...)
		log.Debug(fmt.Sprintf("Bytes of information received: %d", n))

		if len(response) >= responseLength {
			break
		}
	}
	return response, nil
}

func handleConnectionError(err error) bool {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		log.Error("Connection closed because of timeout.")
		return true
	}
	if err == io.EOF {
		log.Error("Connection closed by the server.")
		return true
	}
	return false
}

func validateResponse(response []byte) bool {
	return len(response) >= responseLength
}

func processEnergometerResponse(response []byte, energometer models.Command, conn *net.TCPConn) {
	if !isValidResponse(response, conn) {
		return
	}

	if isErrorResponse(response, conn) {
		return
	}

	date, dateTime := processDate(response, conn)
	if date == "" {
		return
	}

	intSlice := convertResponseToIntSlice(response)
	log.Debug(fmt.Sprintf("Received data from the energometer: %d", intSlice))

	q1 := calculateQ1(response, dateTime)
	if !isValidQ1(q1) {
		q1 = 0
	}

	insertData(q1, energometer, date)

	log.Info(fmt.Sprintf("Date: %s, Command: %s Q1: %f", date, energometer.Current_Data, q1))
}

func isValidResponse(response []byte, conn *net.TCPConn) bool {
	if len(response) < responseLength {
		log.Error("Response is short")
		closeConnectionIfOpen(conn)
		return false
	}
	return true
}

func isErrorResponse(response []byte, conn *net.TCPConn) bool {
	if response[9] == 1 {
		log.Error("This respond has error flag!")
		closeConnectionIfOpen(conn)
		return true
	}

	return false
}

func processDate(response []byte, conn *net.TCPConn) (string, time.Time) {
	date := bytesToDateTime(response[0:6])
	if !checkDate(date) {
		log.Error("Date is wrong! Trying to get the right date...")
		closeConnectionIfOpen(conn)
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
	if q1 > 10000 || q1 < 0 {
		return false
	}
	return true
}

func closeConnectionIfOpen(conn *net.TCPConn) {
	if !isConnectionClosed(conn) {
		conn.Close()
	}
}

func insertData(v1 float32, energometer models.Command, date string) {
	db := ConnectMs()
	defer db.Close()

	_, err := db.Exec(c.GlobalConfig.Query_Insert, energometer.Name, energometer.Id_Measuring, v1, date, 192, nil)
	if err != nil {
		log.Error(fmt.Sprintf("Error during SQL query execution: %s", err.Error()))
	}
}

func ConnectMs() *sql.DB {
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;database=%s",
		c.GlobalConfig.MSSQL.Server,
		c.GlobalConfig.MSSQL.User_Id,
		c.GlobalConfig.MSSQL.Password,
		c.GlobalConfig.MSSQL.Database)

	conn, err := sql.Open("mssql", connString)
	if err != nil {
		log.Error(fmt.Sprintf("Error opening database connection: %s", err.Error()))
		return nil
	}

	if err := conn.Ping(); err != nil {
		log.Error(err.Error())
		time.Sleep(10 * time.Second)
		log.Error("Trying to reconnect to the database...")
		return ConnectMs()
	}

	return conn
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

func processArchives(archiveChan <-chan models.Command) {
	for energometer := range archiveChan {
		retrieveMissingData(energometer)
	}
}

func retrieveMissingData(energometer models.Command) {
	loadTimeFromFile()
	if !shouldRetrieveData(energometer) {
		return
	}

	diff := calculateTimeDifference(energometer)
	if diff <= 1 {
		return
	}

	conn, err := setupConnection(energometer)
	if err != nil {
		return
	}
	defer conn.Close()

	response, err := getInitialResponse(conn, energometer)
	if err != nil {
		return
	}

	processEnergometerResponse(response, energometer, conn)

	retrieveArchiveData(conn, energometer, diff, response)
}

func shouldRetrieveData(energometer models.Command) bool {
	return currentSuccessfulRetrieval[energometer.Current_Data].Year() != 1
}

func calculateTimeDifference(energometer models.Command) int {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	lastRetrieval, exists := lastSuccessfulRetrieval[energometer.Current_Data]
	if !exists {
		lastRetrieval = time.Now().Add(-1 * time.Hour)
	}
	currentRetrieval := currentSuccessfulRetrieval[energometer.Current_Data]

	lastRetrieval = lastRetrieval.Truncate(time.Hour)
	currentRetrieval = currentRetrieval.Truncate(time.Hour)

	diff := int(currentRetrieval.Sub(lastRetrieval).Hours())
	return min(diff, 12)
}

func setupConnection(energometer models.Command) (*net.TCPConn, error) {
	conn, err := createConnection(energometer)
	if err != nil {
		log.Error(fmt.Sprintf("Connection setup failed: %s", err.Error()))
		return nil, err
	}
	return conn, nil
}

func getInitialResponse(conn *net.TCPConn, energometer models.Command) ([]byte, error) {
	if err := sendCommand(conn, energometer.Last_Hour_Archive); err != nil {
		log.Error(fmt.Sprintf("Send command failed: %s", err.Error()))
		return nil, err
	}

	response, err := readResponse(conn)
	if err != nil {
		log.Error(fmt.Sprintf("Read response failed: %s", err.Error()))
		return nil, err
	}
	return response, nil
}

func retrieveArchiveData(conn *net.TCPConn, energometer models.Command, diff int, response []byte) {
	for i := 0; i < diff; i++ {
		if !isValidDateTime(response) {
			continue
		}

		response, err := getBackwardArchive(conn, energometer)
		if err != nil {
			return
		}

		processEnergometerResponse(response, energometer, conn)
	}
}

func isValidDateTime(response []byte) bool {
	date := bytesToDateTime(response[0:6])
	layout := "2006-01-02 15:04:05"
	dateTime, err := time.Parse(layout, date)
	if err != nil {
		log.Error(fmt.Sprintf("Error when converting a string to a date: %s", err))
		return false
	}

	return dateTime.Minute() == 0 && dateTime.Second() == 0
}

func getBackwardArchive(conn *net.TCPConn, energometer models.Command) ([]byte, error) {
	if err := sendCommand(conn, energometer.Backwards_Archive); err != nil {
		log.Error(fmt.Sprintf("Send command failed: %s", err.Error()))
		return nil, err
	}

	response, err := readResponse(conn)
	if err != nil {
		log.Error(fmt.Sprintf("Read response failed: %s", err.Error()))
		return nil, err
	}
	return response, nil
}

func updateSuccessfulRetrieval(command string) {
	retrievalMutex.Lock()
	defer retrievalMutex.Unlock()

	y := currentSuccessfulRetrieval[command].Year()

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
			saveTimeToFile()
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

package main

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"io"
	c "main/configurations"
	"main/models"
	"math"
	"net"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	log "krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging"
)

func main() {
	c.InitConfig()
	log.LogInit(c.GlobalConfig.Log_Level)
	log.Info("Service started!")

	for {
		wait()
		//time.Sleep(1 * time.Minute)

		for _, i := range c.GlobalConfig.Commands {
			//l.Info("Command:", i.Command)
			getData(i)
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

		if err := sendCommand(conn, energometer); err != nil {
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
			return
		} else {
			log.Error(fmt.Sprintf("Received wrong data from the energometer: %v", response))
		}
	}
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

func sendCommand(conn *net.TCPConn, energometer models.Command) error {
	bytecommand := []byte(energometer.Command)

	_, err := conn.Write(bytecommand)
	if err != nil {
		conn.Close()
		return fmt.Errorf("write failed: %w", err)
	}

	log.Info(fmt.Sprintf("Command: %s sent successfully!", energometer.Command))
	return nil
}

func readResponse(conn *net.TCPConn) ([]byte, error) {
	response := make([]byte, 0)
	buffer := make([]byte, 1024)

	conn.SetReadDeadline(time.Now().Add(c.GlobalConfig.Timeout))

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return nil, fmt.Errorf("read timeout: %w", err)
			}
			if err == io.EOF {
				log.Error("Connection closed by the server.")
				break
			}
			return nil, fmt.Errorf("read failed: %w", err)
		}

		response = append(response, buffer[:n]...)
		log.Debug(fmt.Sprintf("Bytes of information received: %d", n))

		if len(response) >= 350 {
			break
		}
	}
	return response, nil
}

func validateResponse(response []byte) bool {
	return len(response) >= 350
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

	intSlice := make([]int, len(response))
	for i, b := range response {
		intSlice[i] = int(b)
	}

	log.Debug(fmt.Sprintf("Received data from the energometer: %d", intSlice))

	q1 := bytesToFloat32(response[24:28])

	if q1 < 0 {
		q1 = 0
	} else if q1 > 10000 {
		getData(energometer)
		if !isConnectionClosed(conn) {
			conn.Close()
		}
		return
	}

	insertData(q1, energometer, date)

	//l.Info("Response:")
	log.Info(fmt.Sprintf("Q1: %f", q1))
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

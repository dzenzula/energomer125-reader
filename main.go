package main

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	c "main/configurations"
	l "main/logger"
	"main/models"
	"math"
	"net"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
)

func main() {
	l.InitLogger()
	c.InitConfig()
	l.Info("Service started!")
	fmt.Println("Service started!")

	for {
		wait()

		for _, i := range c.GlobalConfig.Commands {
			//l.Info("Command:", i.Command)
			getData(i, c.GlobalConfig.Max_Read_Retries)
		}
	}
}

func getData(energometer models.Command, retriesLeft int) {
	if retriesLeft <= 0 {
		l.Error("Reached maximum retries, unable to retrieve valid data.")
		return
	}

	tcpServer, err := net.ResolveTCPAddr(c.GlobalConfig.Connection.Type, c.GlobalConfig.Connection.Host+":"+energometer.Port)
	if err != nil {
		l.Error("ResolveTCPAddr failed:", err.Error())
		getData(energometer, retriesLeft-1)
		return
	}

	conn, err := net.DialTCP(c.GlobalConfig.Connection.Type, nil, tcpServer)
	if err != nil {
		l.Error("Dial failed:", err.Error())
		getData(energometer, retriesLeft-1)
		return
	}

	bytecommand := []byte(energometer.Command)

	_, err = conn.Write(bytecommand)
	if err != nil {
		l.Error("Write failed:", err.Error())
		conn.Close()
		l.Info("Retrying to send the command...")
		getData(energometer, retriesLeft-1)
		return
	} else {
		t := fmt.Sprintf("Command: %s sent successfully!", energometer.Command)
		l.Info(t)
	}

	response := make([]byte, 0)
	buffer := make([]byte, 512)

	timeout := time.AfterFunc(c.GlobalConfig.Timeout, func() {
		l.Error("Timeout on data reading...")
		conn.Close()
	})

	for i := 0; i < 2; i++ {
		n, err := conn.Read(buffer)
		if err != nil {
			fmt.Println("Read failed:", err)
			l.Error("Read failed:", err)
			break
		}

		response = append(response, buffer[:n]...)

		if i == 0 && n == 261 {
			break
		}
	}
	l.Info("Bytes of information recieved:", len(response))
	fmt.Println("Bytes of information recieved:", len(response))
	l.Info("Received data from the energometer:", response)
	fmt.Println("Received data from the energometer:", response)
	timeout.Stop()

	if len(response) >= 261 {
		processEnergometerResponse(response, energometer, conn, retriesLeft)
	} else {
		l.Error("Received wrong data from the energometer:", response)
		l.Info("Trying again to retrieve valid data...")
		conn.Close()
		getData(energometer, retriesLeft-1)
	}

	if !isConnectionClosed(conn) {
		conn.Close()
	}
}

func processEnergometerResponse(response []byte, energometer models.Command, conn *net.TCPConn, retriesLeft int) {
	date := bytesToDateTime(response[0:6])
	if !checkDate(date) {
		l.Error("Date is wrong! Trying to get the right date...")
		l.Info("Response: ", response)
		getData(energometer, retriesLeft-1)
		if !isConnectionClosed(conn) {
			conn.Close()
		}
		return
	}

	q1 := bytesToFloat32(response[24:28])

	if q1 < 0 {
		q1 = 0
	} else if q1 > 10000 {
		getData(energometer, retriesLeft-1)
		if !isConnectionClosed(conn) {
			conn.Close()
		}
		return
	}

	insertData(q1, energometer, date)

	//l.Info("Response:")
	l.Info("Q1:", q1)
}

func insertData(v1 float32, energometr models.Command, date string) {
	db := ConnectMs()
	q := c.GlobalConfig.Query_Insert

	_, err := db.Exec(q, energometr.Name, energometr.Id_Measuring, v1, date, 192, nil)
	if err != nil {
		l.Error("Error during SQL query execution:", err.Error())
	}

	defer db.Close()
}

func ConnectMs() *sql.DB {
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;database=%s", c.GlobalConfig.MSSQL.Server, c.GlobalConfig.MSSQL.User_Id, c.GlobalConfig.MSSQL.Password, c.GlobalConfig.MSSQL.Database)
	conn, conErr := sql.Open("mssql", connString)
	if conErr != nil {
		l.Error("Error opening database connection:", conErr.Error())
	}

	pingErr := conn.Ping()
	if pingErr != nil {
		l.Error(pingErr.Error())
		time.Sleep(10 * time.Second)
		l.Info("Trying to reconnect to the database...")
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
	t := time.Now().Add(duration).Format("2006-01-02 15:04:05")

	l.Info("Time until the next iteration:", t)
	fmt.Println("Time until the next iteration:", t)

	time.Sleep(duration)
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
		l.Error("Error when converting a string to a date:", err)
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

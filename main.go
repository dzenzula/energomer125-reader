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
	"strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
)

func main() {
	l.InitLogger()
	c.InitConfig()
	l.Info("Service started!")

	for {
		//waitHalfHour()
		waitTenMinutes()

		for _, c := range c.GlobalConfig.Commands {
			l.Info("Command:", c.Command)
			getData(c)
		}
	}
}

func getData(energometer models.Command) {
	tcpServer, err := net.ResolveTCPAddr(c.GlobalConfig.Connection.Type, c.GlobalConfig.Connection.Host+":"+c.GlobalConfig.Connection.Port)
	if err != nil {
		l.Error("ResolveTCPAddr failed:", err.Error())
		return
	}

	conn, err := net.DialTCP(c.GlobalConfig.Connection.Type, nil, tcpServer)
	if err != nil {
		l.Error("Dial failed:", err.Error())
		return
	} else {
		l.Info("Dial success!")
	}

	bytecommand := []byte(energometer.Command)

	_, err = conn.Write(bytecommand)
	if err != nil {
		l.Error("Write failed:", err.Error())
		return
	} else {
		l.Info("Command sent successfully!")
	}

	response := make([]byte, 512)
	conn.Read(response[:])
	date := bytesToDateTime(response[0:6])
	v1 := bytesToFloat32(response[14:18])
	v2 := bytesToFloat32(response[18:22])
	// v3 := bytesToFloat32(response[22:26])
	// v4 := bytesToFloat32(response[26:30])

	contains := strings.Contains(energometer.Command, "7825")

	if contains && c.GlobalConfig.V1 == int(v1) && c.GlobalConfig.V2 == int(v2) {
		insertData(v1, energometer, date)
	} else if !contains && c.GlobalConfig.V1 == int(v1) && c.GlobalConfig.V2 == int(v2) {
		conn.Close()
		getData(energometer)
		return
	} else {
		insertData(v1, energometer, date)
	}

	//l.Info("Response:", response)

	conn.Close()
}

func insertData(v1 float32, energometr models.Command, date string) {
	db := ConnectMs()

	_, err := db.Exec(c.GlobalConfig.QueryInsert, energometr.Name, energometr.IDMeasuring, v1, date, 192, nil)
	if err != nil {
		l.Error("Ошибка при выполнении SQL-запроса:", err.Error())
	}

	db.Close()
}

func ConnectMs() *sql.DB {
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;database=%s", c.GlobalConfig.MSSQL.Server, c.GlobalConfig.MSSQL.UserID, c.GlobalConfig.MSSQL.Password, c.GlobalConfig.MSSQL.Database)
	conn, conErr := sql.Open("mssql", connString)
	if conErr != nil {
		l.Error("Error opening database connection:", conErr.Error())
	}

	pingErr := conn.Ping()
	if pingErr != nil {
		l.Error(pingErr.Error())
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

func waitTenMinutes() {
	now := time.Now()

	minutesRemaining := 10 - (now.Minute() % 10)

	nextTime := now.Add(time.Duration(minutesRemaining) * time.Minute)

	l.Info("Time until the next 10 minutes:", nextTime)

	time.Sleep(time.Until(nextTime))
}

// func waitHalfHour() {
// 	now := time.Now()
// 	minutes := now.Minute()
// 	seconds := now.Second()

// 	var waitSeconds int
// 	if minutes >= 30 {
// 		waitSeconds = (60 - minutes) * 60
// 	} else {
// 		waitSeconds = (30 - minutes) * 60
// 	}

// 	nextTime := now.Add(time.Duration(waitSeconds-seconds) * time.Second)

// 	l.Info("Time until the next 30 minutes:", nextTime)

// 	time.Sleep(time.Until(nextTime))
// }

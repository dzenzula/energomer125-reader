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

		for _, c := range c.GlobalConfig.Commands {
			l.Info("Command:", c.Command)
			getData(c)
		}
	}
}

func getData(energometer models.Command) {
	tcpServer, err := net.ResolveTCPAddr(c.GlobalConfig.Connection.Type, c.GlobalConfig.Connection.Host+":"+energometer.Port)
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

	response := make([]byte, 0)
	buffer := make([]byte, 512)

	timeout := time.AfterFunc(120*time.Second, func() {
		conn.Close()
		l.Error("Timeout при чтении данных")
		getData(energometer)
		return
	})
	for {
		n, err := conn.Read(buffer)
		if err != nil {
			fmt.Println("Read failed:", err)
			l.Error("Read failed:", err)
			break
		}

		response = append(response, buffer[:n]...)

		if n < len(buffer) {
			break
		}
	}
	timeout.Stop()

	date := bytesToDateTime(response[0:6])
	q1 := bytesToFloat32(response[24:28])

	if len(response) < 262 {
		insertData(q1, energometer, date)
	}

	l.Info("Response:")
	fmt.Println("Command:", energometer.Command)
	fmt.Println("Date:", date)
	fmt.Println("Response:", response)
	fmt.Println("Q1:", q1)
	l.Info("Q1:", q1)

	conn.Close()
}

func insertData(v1 float32, energometr models.Command, date string) {
	db := ConnectMs()
	q := c.GlobalConfig.Query_Insert

	_, err := db.Exec(q, energometr.Name, energometr.Id_Measuring, v1, date, 192, nil)
	if err != nil {
		l.Error("Error during SQL query execution:", err.Error())
	}

	db.Close()
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

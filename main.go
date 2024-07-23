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
	log.LogInit(c.GlobalConfig.LogLevel)
	c.InitConfig()
	log.Info("Service started!")

	for {
		wait()
		//time.Sleep(1 * time.Minute)

		for _, i := range c.GlobalConfig.Commands {
			//l.Info("Command:", i.Command)
			getData(i)
		}
	}
}

func getData(energometer models.Command) {
	for i := 0; i < c.GlobalConfig.Max_Read_Retries; i++ {
		tcpServer, err := net.ResolveTCPAddr(c.GlobalConfig.Connection.Type, c.GlobalConfig.Connection.Host+":"+energometer.Port)
		if err != nil {
			log.Error(fmt.Sprintf("ResolveTCPAddr failed: %s", err.Error()))
			continue
		}

		conn, err := net.DialTCP(c.GlobalConfig.Connection.Type, nil, tcpServer)
		if err != nil {
			log.Error(fmt.Sprintf("Dial failed: %s", err.Error()))
			continue
		}

		bytecommand := []byte(energometer.Command)

		_, err = conn.Write(bytecommand)
		if err != nil {
			log.Error(fmt.Sprintf("Write failed: %s", err.Error()))
			conn.Close()
			log.Info("Retrying to send the command...")
			continue
		} else {
			t := fmt.Sprintf("Command: %s sent successfully!", energometer.Command)
			log.Info(t)
		}

		response := make([]byte, 0)
		buffer := make([]byte, 1024)

		/*timeout := time.AfterFunc(c.GlobalConfig.Timeout, func() {
			log.Error("Timeout on data reading...\n Trying ")
			conn.Close()
		})*/

		for {
			n, err := conn.Read(buffer)
			if err != nil {
				if err == io.EOF {
					log.Info("Connection closed by the server.")
					break
				}
				log.Error(fmt.Sprintf("Read failed: %s", err))
				conn.Close()
				log.Info("Retrying to retrieve valid data...")
				continue
			}

			response = append(response, buffer[:n]...)
			log.Info(fmt.Sprintf("Bytes of information recieved: %d", n))

			if len(response) >= 130 {
				break
			}
		}
		//timeout.Stop()

		if len(response) >= 130 {
			processEnergometerResponse(response, energometer, conn)
		} else {
			intSlice := make([]int, len(response))
			for i, b := range response {
				intSlice[i] = int(b)
			}

			log.Error(fmt.Sprintf("Received wrong data from the energometer: %d", intSlice))
			log.Info("Trying again to retrieve valid data...")
			conn.Close()
			continue
		}

		if i == 2 {
			log.Error("Reached maximum retries, unable to retrieve valid data.")
			conn.Close()
			return
		}

		conn.Close()
		break
	}
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

	log.Error(fmt.Sprintf("Received data from the energometer: %d", intSlice))

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
		log.Info("Trying to reconnect to the database...")
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

	log.Info(fmt.Sprintf("Time until the next iteration: %s", t))
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

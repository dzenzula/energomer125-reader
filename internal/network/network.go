package network

import (
	"cmd/energomer125-reader/internal/config"
	"cmd/energomer125-reader/internal/models"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	log "krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging"
)

type ConnectionManager struct {
	conn *net.TCPConn
	mu   sync.Mutex
}

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{}
}

func (cm *ConnectionManager) SetupConnection(energomer models.Command) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	tcpServer, err := net.ResolveTCPAddr(config.GlobalConfig.Connection.Type, fmt.Sprintf("%s:%s", config.GlobalConfig.Connection.Host, energomer.Port))
	if err != nil {
		return fmt.Errorf("resolveTCPAddr failed: %w", err)
	}

	conn, err := net.DialTCP(config.GlobalConfig.Connection.Type, nil, tcpServer)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	cm.conn = conn
	return nil
}

func (cm *ConnectionManager) SendCommand(command string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.conn == nil {
		return fmt.Errorf("connection is not established")
	}

	_, err := cm.conn.Write([]byte(command))
	if err != nil {
		cm.conn.Close()
		cm.conn = nil
		return fmt.Errorf("write failed: %w", err)
	}

	log.Info(fmt.Sprintf("Command: %s sent successfully!", command))
	return nil
}

func (cm *ConnectionManager) ReadResponse(command string) ([]byte, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.conn == nil {
		return nil, fmt.Errorf("connection is not established")
	}

	response := make([]byte, 0)
	buffer := make([]byte, 4096)

	cm.conn.SetReadDeadline(time.Now().Add(config.GlobalConfig.Timeout))

	for {
		n, err := cm.conn.Read(buffer)
		if err != nil {
			if cm.handleConnectionError(err) {
				break
			}
			return nil, fmt.Errorf("read failed: %w", err)
		}

		response = append(response, buffer[:n]...)
		log.Debug(fmt.Sprintf("Bytes of information received: %d", n))

		if len(response) >= cm.getResponseLength(command) {
			break
		}
	}
	return response, nil
}

func (cm *ConnectionManager) CloseConnection() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.conn != nil {
		err := cm.conn.Close()
		if err != nil {
			log.Error(fmt.Sprintf("Error closing connection: %s", err))
			return
		}
		cm.conn = nil
		log.Info("Connection closed successfully.")
	}
}

func (cm *ConnectionManager) handleConnectionError(err error) bool {
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

func (cm *ConnectionManager) getResponseLength(command string) int {
	for _, c := range config.GlobalConfig.Commands {
		if command == c.Current_Data {
			return 336
		}
	}
	return 132
}

package database

import (
	"cmd/energomer125-reader/internal/config"
	"cmd/energomer125-reader/internal/models"

	"database/sql"
	"fmt"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
	log "krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging"
)

func connectMs() (*sql.DB, error) {
	retries := config.GlobalConfig.Max_Read_Retries
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;database=%s",
		config.GlobalConfig.MSSQL.Server,
		config.GlobalConfig.MSSQL.User_Id,
		config.GlobalConfig.MSSQL.Password,
		config.GlobalConfig.MSSQL.Database)

	conn, err := sql.Open("mssql", connString)
	if err != nil {
		log.Error(fmt.Sprintf("Error opening database connection: %s", err.Error()))
		return nil, err
	}

	for retries > 0 {
		err = conn.Ping()
		if err == nil {
			log.Debug("Successfully connected to the database.")
			return conn, nil
		}

		log.Error(fmt.Sprintf("Database connection ping failed: %s", err.Error()))
		retries--

		if retries > 0 {
			time.Sleep(10 * time.Second)
			log.Info("Retrying database connection...")
		}
	}

	return nil, fmt.Errorf("failed to connect to the database after multiple attempts: %w", err)
}

func InsertData(v1 float32, energomer models.Command, date string) error {
	db, err := connectMs()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(config.GlobalConfig.Query_Insert, energomer.Name, energomer.Id_Measuring, v1, date, 192, nil)
	if err != nil {
		log.Error(fmt.Sprintf("Error during SQL query execution: %s", err.Error()))
		return err
	}

	return nil
}

package models

import "time"

type MSSQLConfig struct {
	Server   string `yaml:"server"`
	User_Id  string `yaml:"user_id"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type Command struct {
	Current_Data      string `yaml:"current_data"`
	Last_Hour_Archive string `yaml:"last_hour_archive"`
	Forward_Archive   string `yaml:"forward_archive"`
	Backwards_Archive string `yaml:"backwards_archive"`
	Port              string `yaml:"port"`
	Id_Measuring      int    `yaml:"id_measuring"`
	Name              string `yaml:"name"`
}

type Connection struct {
	Host string `yaml:"host"`
	Type string `yaml:"type"`
}

type Configuration struct {
	MSSQL            MSSQLConfig   `yaml:"mssql"`
	Commands         []Command     `yaml:"commands"`
	Connection       Connection    `yaml:"connection"`
	Query_Insert     string        `yaml:"query_insert"`
	Timer            time.Duration `yaml:"timer"`
	Timeout          time.Duration `yaml:"timeout"`
	Max_Read_Retries int           `yaml:"max_read_retries"`
	Log_Level        string        `yaml:"log_level"`
}

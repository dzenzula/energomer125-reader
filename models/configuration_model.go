package models

import "time"

type MSSQLConfig struct {
	Server   string `yaml:"server"`
	User_Id  string `yaml:"user_id"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type Command struct {
	Command      string `yaml:"command"`
	Port         string `yaml:"port"`
	Id_Measuring int    `yaml:"id_measuring"`
	Name         string `yaml:"name"`
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
	Log_Level         string        `yaml:"log_level"`
}

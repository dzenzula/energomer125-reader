package models

import "time"

type MSSQLConfig struct {
	Server   string `yaml:"server"`
	UserID   string `yaml:"user_id"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type Command struct {
	Command     string `yaml:"command"`
	Port        string `yaml:"port"`
	IDMeasuring int    `yaml:"id_measuring"`
	Name        string `yaml:"name"`
}

type Connection struct {
	Host string `yaml:"host"`
	Type string `yaml:"type"`
}

type Configuration struct {
	MSSQL       MSSQLConfig   `yaml:"mssql"`
	Commands    []Command     `yaml:"commands"`
	Connection  Connection    `yaml:"connection"`
	QueryInsert string        `yaml:"query_insert"`
	Timer       time.Duration `yaml:"timer"`
}

package models

type MSSQLConfig struct {
	Server   string `yaml:"server"`
	UserID   string `yaml:"user_id"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type Command struct {
	Command     string `yaml:"command"`
	IDMeasuring int    `yaml:"id_measuring"`
	Name        string `yaml:"name"`
}

type Connection struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
	Type string `yaml:"type"`
}

type Configuration struct {
	MSSQL       MSSQLConfig `yaml:"mssql"`
	Commands    []Command   `yaml:"commands"`
	Connection  Connection  `yaml:"connection"`
	V1          int         `yaml:"v1"`
	V2          int         `yaml:"v2"`
	V3          int         `yaml:"v3"`
	V4          int         `yaml:"v4"`
	QueryInsert string      `yaml:"query_insert"`
}

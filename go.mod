module main

go 1.22.2

require (
	github.com/denisenkom/go-mssqldb v0.12.3
	krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging v0.0.0
)

require (
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/pelletier/go-toml/v2 v2.0.8 // indirect
	github.com/spf13/afero v1.9.5 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.4.2 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

require (
	github.com/fsnotify/fsnotify v1.6.0
	github.com/golang-sql/civil v0.0.0-20190719163853-cb61b32ac6fe // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/viper v1.16.0
	golang.org/x/crypto v0.9.0 // indirect
)

replace krr-app-gitlab01.europe.mittalco.com/pait/modules/go/logging => ./src/logging

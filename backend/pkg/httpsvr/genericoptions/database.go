package genericoptions

import (
	"fmt"

	"github.com/spf13/pflag"
)

type DatabaseOptions struct {
	Type        string             `json:"type,omitempty" mapstructure:"type"`
	PostgresSQL PostgresSQLOptions `json:"postgresql"     mapstructure:"postgresql"`
	MySQL       MySQLOptions       `json:"mysql"          mapstructure:"mysql"`
}

// NewDatabaseOptions create a `zero` value instance.
func NewDatabaseOptions() *DatabaseOptions {
	return &DatabaseOptions{}
}

// Validate verifies flags passed to DatabaseOptions.
func (o *DatabaseOptions) Validate() []error {
	errs := []error{}

	switch o.Type {
	case "postgresql":
		perr := o.PostgresSQL.Validate()
		if perr != nil {
			errs = append(errs, perr...)
		}
	case "mysql":
		perr := o.MySQL.Validate()
		if perr != nil {
			errs = append(errs, perr...)
		}

	case "":
		errs = append(errs, fmt.Errorf("--database.type must be provided to choose which data base use"))
	default:
		errs = append(errs, fmt.Errorf("--database.type only support postgresql|mysql"))
	}

	return errs
}

// AddFlags adds flags related to postgresql storage for a specific APIServer to the specified FlagSet.
func (o *DatabaseOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.Type, "database.type", o.Type, ""+
		"Choose using database to use?")

	fs.StringVar(&o.MySQL.Host, "database.mysql.host", o.MySQL.Host, ""+
		"MySQL service host address. If left blank, the following related database.mysql options will be ignored.")

	fs.StringVar(&o.MySQL.Username, "database.mysql.username", o.MySQL.Username, ""+
		"Username for access to mysql service.")

	fs.StringVar(&o.MySQL.Password, "database.mysql.password", o.MySQL.Password, ""+
		"Password for access to mysql, should be used pair with password.")

	fs.StringVar(&o.MySQL.Database, "database.mysql.database", o.MySQL.Database, ""+
		"Database name for the server to use.")

	fs.IntVar(&o.MySQL.MaxIdleConnections, "database.mysql.max-idle-connections", o.MySQL.MaxOpenConnections, ""+
		"Maximum idle connections allowed to connect to mysql.")

	fs.IntVar(&o.MySQL.MaxOpenConnections, "database.mysql.max-open-connections", o.MySQL.MaxOpenConnections, ""+
		"Maximum open connections allowed to connect to mysql.")

	fs.DurationVar(
		&o.MySQL.MaxConnectionLifeTime,
		"database.mysql.max-connection-life-time",
		o.MySQL.MaxConnectionLifeTime,
		""+
			"Maximum connection life time allowed to connect to mysql.",
	)

	fs.IntVar(&o.MySQL.LogLevel, "database.mysql.log-mode", o.MySQL.LogLevel, ""+
		"Specify gorm log level.")

	fs.StringVar(&o.PostgresSQL.Host, "database.postgresql.host", o.PostgresSQL.Host, ""+
		"PostgresSQL service host address. If left blank, the following related postgresql options will be ignored.")

	fs.IntVar(&o.PostgresSQL.Port, "database.postgresql.port", o.PostgresSQL.Port, ""+
		"PostgresSQL service host port. If left blank, will use default 5432 ")

	fs.StringVar(&o.PostgresSQL.Username, "database.postgresql.username", o.PostgresSQL.Username, ""+
		"Username for access to postgresql service.")

	fs.StringVar(&o.PostgresSQL.Password, "database.postgresql.password", o.PostgresSQL.Password, ""+
		"Password for access to postgresql, should be used pair with password.")

	fs.StringVar(&o.PostgresSQL.Database, "database.postgresql.database", o.PostgresSQL.Database, ""+
		"Database name for the server to use.")

	fs.IntVar(
		&o.PostgresSQL.MaxIdleConnections,
		"database.postgresql.max-idle-connections",
		o.PostgresSQL.MaxOpenConnections,
		""+
			"Maximum idle connections allowed to connect to postgresql.",
	)

	fs.IntVar(
		&o.PostgresSQL.MaxOpenConnections,
		"database.postgresql.max-open-connections",
		o.PostgresSQL.MaxOpenConnections,
		""+
			"Maximum open connections allowed to connect to postgresql.",
	)

	fs.DurationVar(
		&o.PostgresSQL.MaxConnectionLifeTime,
		"database.postgresql.max-connection-life-time",
		o.PostgresSQL.MaxConnectionLifeTime,
		""+
			"Maximum connection life time allowed to connect to postgresql.",
	)

	fs.IntVar(&o.PostgresSQL.LogLevel, "database.postgresql.log-mode", o.PostgresSQL.LogLevel, ""+
		"Specify gorm log level.")

	fs.BoolVar(&o.PostgresSQL.StartAlive, "database.postgresql.start-alive", o.PostgresSQL.StartAlive,
		"Specify postgresql connect must be alive during start up")
}

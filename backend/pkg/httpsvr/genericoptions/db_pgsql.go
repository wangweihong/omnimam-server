package genericoptions

import (
	"fmt"
	"time"

	"github.com/wangweihong/gotoolbox/pkg/errors"

	"github.com/wangweihong/omnimam/backend/pkg/httpsvr/genericoptions/postgresqldb"

	"github.com/spf13/pflag"
	"gorm.io/gorm"
)

// PostgresSQLOptions defines options for postgresql database.
type PostgresSQLOptions struct {
	Host                  string        `json:"host,omitempty"                     mapstructure:"host"`
	Port                  int           `json:"port,omitempty"                     mapstructure:"port"`
	Username              string        `json:"username,omitempty"                 mapstructure:"username"`
	Password              string        `json:"-"                                  mapstructure:"password"`
	Database              string        `json:"database"                           mapstructure:"database"`
	MaxIdleConnections    int           `json:"max-idle-connections,omitempty"     mapstructure:"max-idle-connections"`
	MaxOpenConnections    int           `json:"max-open-connections,omitempty"     mapstructure:"max-open-connections"`
	MaxConnectionLifeTime time.Duration `json:"max-connection-life-time,omitempty" mapstructure:"max-connection-life-time"`
	LogLevel              int           `json:"log-level"                          mapstructure:"log-level"`
	StartAlive            bool          `json:"start-alive"                        mapstructure:"start-alive"`
}

// NewPostgresSQLOptions create a `zero` value instance.
func NewPostgresSQLOptions() *PostgresSQLOptions {
	return &PostgresSQLOptions{
		Host:                  "127.0.0.1",
		Port:                  5432,
		Username:              "",
		Password:              "",
		Database:              "",
		MaxIdleConnections:    100,
		MaxOpenConnections:    100,
		MaxConnectionLifeTime: time.Duration(10) * time.Second,
		LogLevel:              1, // Silent
		StartAlive:            true,
	}
}

// Validate verifies flags passed to PostgresSQLOptions.
func (o *PostgresSQLOptions) Validate() []error {
	errs := []error{}

	if o.Username == "" || o.Password == "" {
		errs = append(errs, fmt.Errorf("--postgresql.username and --postgresql.password must be provided"))
	}

	if o.Host == "" {
		errs = append(errs, fmt.Errorf("--postgresql.host must be provided"))
	}

	if o.Port < 0 || o.Port > 65535 {
		errs = append(
			errs,
			fmt.Errorf(
				"--postgresql.port %v must be between 0 and 65535, inclusive",
				o.Port,
			),
		)
	}

	if o.Database == "" {
		errs = append(errs, fmt.Errorf("--postgresql.database must be provided"))
	}

	return errs
}

// AddFlags adds flags related to postgresql storage for a specific APIServer to the specified FlagSet.
func (o *PostgresSQLOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.Host, "postgresql.host", o.Host, ""+
		"PostgresSQL service host address. If left blank, the following related postgresql options will be ignored.")

	fs.IntVar(&o.Port, "postgresql.port", o.Port, ""+
		"PostgresSQL service host port. If left blank, will use default 5432 ")

	fs.StringVar(&o.Username, "postgresql.username", o.Username, ""+
		"Username for access to postgresql service.")

	fs.StringVar(&o.Password, "postgresql.password", o.Password, ""+
		"Password for access to postgresql, should be used pair with password.")

	fs.StringVar(&o.Database, "postgresql.database", o.Database, ""+
		"Database name for the server to use.")

	fs.IntVar(&o.MaxIdleConnections, "postgresql.max-idle-connections", o.MaxOpenConnections, ""+
		"Maximum idle connections allowed to connect to postgresql.")

	fs.IntVar(&o.MaxOpenConnections, "postgresql.max-open-connections", o.MaxOpenConnections, ""+
		"Maximum open connections allowed to connect to postgresql.")

	fs.DurationVar(&o.MaxConnectionLifeTime, "postgresql.max-connection-life-time", o.MaxConnectionLifeTime, ""+
		"Maximum connection life time allowed to connect to postgresql.")

	fs.IntVar(&o.LogLevel, "postgresql.log-mode", o.LogLevel, ""+
		"Specify gorm log level.")

	fs.BoolVar(&o.StartAlive, "postgresql.start-alive", o.StartAlive,
		"Specify postgresql connect must be alive during start up")
}

// NewClient create postgresql store with the given config.
func (o *PostgresSQLOptions) NewClient() (*gorm.DB, error) {
	opts := &postgresqldb.Options{
		Host:                  o.Host,
		Username:              o.Username,
		Password:              o.Password,
		Database:              o.Database,
		MaxIdleConnections:    o.MaxIdleConnections,
		MaxOpenConnections:    o.MaxOpenConnections,
		MaxConnectionLifeTime: o.MaxConnectionLifeTime,
		LogLevel:              o.LogLevel,
		Port:                  o.Port,
	}

	db, err := postgresqldb.New(opts)
	if err != nil {
		return nil, errors.Errorf("new postgresql db err:%v", err)
	}
	return db, nil
}

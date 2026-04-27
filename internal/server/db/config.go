package db

import "time"

type Config struct {
	Dialect         string        `conf:"dialect" yaml:"dialect" json:"dialect"`
	DSN             string        `conf:"dsn" yaml:"dsn" json:"dsn"`
	Debug           bool          `conf:"debug" yaml:"debug" json:"debug"`
	MaxOpenConns    int           `conf:"max_open_conns" yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int           `conf:"max_idle_conns" yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime time.Duration `conf:"conn_max_lifetime" yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `conf:"conn_max_idle_time" yaml:"conn_max_idle_time" json:"conn_max_idle_time"`

	ReadReplica ReadReplicaConfig `conf:"read_replica" yaml:"read_replica" json:"read_replica"`
}

type ReadReplicaConfig struct {
	DSN          string `conf:"read_dsn" yaml:"read_dsn" json:"read_dsn"`
	MaxOpenConns int    `conf:"read_max_open_conns" yaml:"read_max_open_conns" json:"read_max_open_conns"`
	MaxIdleConns int    `conf:"read_max_idle_conns" yaml:"read_max_idle_conns" json:"read_max_idle_conns"`
}

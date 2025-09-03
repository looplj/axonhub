package db

type Config struct {
	Dialect string `conf:"dialect" yaml:"dialect" json:"dialect"`
	DSN     string `conf:"dsn" yaml:"dsn" json:"dsn"`
	Debug   bool   `conf:"debug" yaml:"debug" json:"debug"`
}

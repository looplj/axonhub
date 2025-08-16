package db

type Config struct {
	Dialect string `conf:"dialect"`
	DSN     string `conf:"dsn"`
	Debug   bool   `conf:"debug"`
}

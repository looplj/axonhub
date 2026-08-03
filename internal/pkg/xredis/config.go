package xredis

import (
	"time"
)

type Config struct {
	Addr                  string        `conf:"addr" yaml:"addr" json:"addr"`
	URL                   string        `conf:"url" yaml:"url" json:"url"`
	Addrs                 []string      `conf:"addrs" yaml:"addrs" json:"addrs"`
	Username              string        `conf:"username" yaml:"username" json:"username"`
	Password              string        `conf:"password" yaml:"password" json:"password"`
	MasterName            string        `conf:"master_name" yaml:"master_name" json:"master_name"`
	SentinelUsername      string        `conf:"sentinel_username" yaml:"sentinel_username" json:"sentinel_username"`
	SentinelPassword      string        `conf:"sentinel_password" yaml:"sentinel_password" json:"sentinel_password"`
	RouteByLatency        *bool         `conf:"route_by_latency" yaml:"route_by_latency" json:"route_by_latency"`
	RouteRandomly         *bool         `conf:"route_randomly" yaml:"route_randomly" json:"route_randomly"`
	IsClusterMode         *bool         `conf:"is_cluster_mode" yaml:"is_cluster_mode" json:"is_cluster_mode"`
	DB                    *int          `conf:"db" yaml:"db" json:"db"`
	TLS                   bool          `conf:"tls" yaml:"tls" json:"tls"`
	TLSInsecureSkipVerify bool          `conf:"tls_insecure_skip_verify" yaml:"tls_insecure_skip_verify" json:"tls_insecure_skip_verify"`
	Expiration            time.Duration `conf:"expiration" yaml:"expiration" json:"expiration"`
}

// Package config loads the standalone product's configuration without starting
// a runtime, opening storage, or making network requests.
package config

import (
	"crypto/tls"
	"net/url"
	"time"
	_ "time/tzdata" // Keep IANA locations available without system zoneinfo.
)

type StorageType string

const (
	StorageBbolt StorageType = "bbolt"
	StorageRedis StorageType = "redis"
)

// CalendarDays is a positive calendar-day span, not a multiple of 24 hours.
type CalendarDays int

// Config contains only validated, active configuration. It must not be logged
// or dumped: URLs, headers, and authentication fields may contain secrets.
type Config struct {
	Server    ServerConfig
	Storage   StorageConfig
	Uptime    UptimeConfig
	UI        UIConfig
	Auth      *BasicAuthConfig
	Endpoints []EndpointConfig
}

type ServerConfig struct {
	Address         string
	ShutdownTimeout time.Duration
	TLS             *TLSConfig // nil means disabled.
}

type TLSConfig struct {
	CertFile    string
	KeyFile     string
	Certificate tls.Certificate
}

type StorageConfig struct {
	Type  StorageType
	Bbolt *BboltConfig
	Redis *RedisConfig
}

type BboltConfig struct{ Path string }

type RedisConfig struct {
	URL       *url.URL
	KeyPrefix string
}

type UptimeConfig struct {
	Interval  time.Duration
	Retention CalendarDays
	Window    CalendarDays
	Timezone  *time.Location
}

type UIConfig struct {
	Path        string
	Title       string
	Description string
	Footer      string
	FaviconURL  *url.URL // nil selects the built-in favicon.
	Thresholds  ThresholdConfig
}

type ThresholdConfig struct{ Green, Yellow float64 }

type BasicAuthConfig struct{ Username, PasswordHash string }

type EndpointConfig struct {
	ID                  string
	Name                string
	Description         string
	URL                 *url.URL
	Method              string
	Interval            time.Duration
	Timeout             time.Duration
	Headers             map[string]string
	ExpectedStatusCodes []int // Empty selects the upstream 2xx/3xx policy.
}

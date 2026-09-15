package config

import "time"

// Constitution defaults, exported for tests and documentation.
const (
	DefaultMaxRequestBytes      int64         = 1 << 20
	DefaultMaxHeaderBytes       int           = 8 << 10
	DefaultRequestTimeout       time.Duration = 30 * time.Second
	DefaultIdleTimeout          time.Duration = 60 * time.Second
	DefaultHandshakeTimeout     time.Duration = 10 * time.Second
	DefaultMaxConcurrentStreams uint32        = 100
)

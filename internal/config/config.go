package config

import "time"

type (
	Config struct {
		App      App      `env-prefix:"APP_"`
		Service  Service  `env-prefix:"SERVICE_"`
		Database Database `env-prefix:"DB_"`
		Cache    Cache    `env-prefix:"CACHE_"`
		HTTP     HTTP     `env-prefix:"HTTP_"`
		Logger   Logger   `env-prefix:"LOGGER_"`
		Env      string   `env:"ENV" env-default:"local" validate:"oneof=local dev staging prod"`
	}

	App struct {
		Name    string
		Version string
	}

	Service struct {
		MaxDepth        int
		DefaultPageSize int
		MaxPageSize     int
	}

	Database struct {
		DSN            string
		PoolMax        int32
		ConnAttempts   int
		BaseRetryDelay time.Duration
		MaxRetryDelay  time.Duration
	}

	Cache struct {
		Addr        string
		Password    string
		PoolSize    int
		MinIdleCons int
		PoolTimeout time.Duration
		TTL         time.Duration
	}

	HTTP struct {
		Host              string
		Port              string
		ReadTimeout       time.Duration
		WriteTimeout      time.Duration
		IdleTimeout       time.Duration
		ShutdownTimeout   time.Duration
		ReadHeaderTimeout time.Duration
	}

	Logger struct {
		Level      string
		Filename   string
		MaxSize    int
		MaxBackups int
		MaxAge     int
	}
)

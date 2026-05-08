package config

import (
	"github.com/ilyakaznacheev/cleanenv"
	"go.uber.org/zap"
	"sync"
)

type Config struct {
	AppPort  string `env:"APP_PORT" env-default:"8080"`
	LogLevel string `env:"LOG_LEVEL" env-default:"info"`
	KafkaURL string `env:"KAFKA_URL" env-default:"localhost:9092"`
	Topic    string `env:"KAFKA_TOPIC" env-default:"ax-telemetry"`
	RedisURL string `env:"REDIS_URL" env-default:"localhost:6379"`
}

var (
	instance *Config
	once     sync.Once
	Logger   *zap.Logger // Global enterprise logger
)

// InitConfigAndLogger initializes config and structured logging.
func InitConfigAndLogger() *Config {
	once.Do(func() {
		instance = &Config{}
		if err := cleanenv.ReadEnv(instance); err != nil {
			panic("failed to read configuration: " + err.Error())
		}

		// Initialize Uber Zap for JSON structured logging
		var err error
		if instance.LogLevel == "debug" {
			Logger, err = zap.NewDevelopment()
		} else {
			Logger, err = zap.NewProduction()
		}
		if err != nil {
			panic("failed to initialize logger: " + err.Error())
		}
	})
	return instance
}

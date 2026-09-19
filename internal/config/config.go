package config

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Options are the per-service inputs to Load. Each service binary passes its own
// identity and default port; everything else is shared infrastructure config.
type Options struct {
	ServiceName string
	DefaultPort string
}

// Config is the fully-resolved application configuration. It is built once by
// Load at startup and then passed down by dependency injection; nothing
// downstream reads the environment directly.
type Config struct {
	App      AppConfig
	Postgres PostgresConfig
	Kafka    KafkaConfig
	Gateway  GatewayConfig
}

type AppConfig struct {
	Name string
	Port string
}

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

// DSN returns a lib/pq connection string.
func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

type KafkaConfig struct {
	Brokers []string
}

// GatewayConfig points the worker at the payment gateway it charges against.
// In this project that is the fake-gateway container.
type GatewayConfig struct {
	URL string
}

// Load resolves configuration a single time. Precedence, highest first:
// real environment variables, then the optional .env file, then defaults.
func Load(opts Options) (*Config, error) {
	v := viper.New()

	v.SetConfigFile(".env")
	v.SetConfigType("env")
	// .env is optional: under Docker the values arrive as real env vars.
	if err := v.ReadInConfig(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read .env: %w", err)
	}
	v.AutomaticEnv()

	v.SetDefault("APP_PORT", opts.DefaultPort)
	v.SetDefault("DB_HOST", "localhost")
	v.SetDefault("DB_PORT", "5432")
	v.SetDefault("DB_USER", "postgres")
	v.SetDefault("DB_PASSWORD", "postgres")
	v.SetDefault("DB_NAME", "design-payment-system")
	v.SetDefault("DB_SSLMODE", "disable")
	v.SetDefault("KAFKA_BROKERS", "localhost:29092")
	v.SetDefault("GATEWAY_URL", "http://localhost:8082")

	cfg := &Config{
		App: AppConfig{
			Name: opts.ServiceName,
			Port: v.GetString("APP_PORT"),
		},
		Postgres: PostgresConfig{
			Host:     v.GetString("DB_HOST"),
			Port:     v.GetString("DB_PORT"),
			User:     v.GetString("DB_USER"),
			Password: v.GetString("DB_PASSWORD"),
			Name:     v.GetString("DB_NAME"),
			SSLMode:  v.GetString("DB_SSLMODE"),
		},
		Kafka: KafkaConfig{
			Brokers: splitAndTrim(v.GetString("KAFKA_BROKERS")),
		},
		Gateway: GatewayConfig{
			URL: strings.TrimSpace(v.GetString("GATEWAY_URL")),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.App.Name == "" {
		return errors.New("service name must not be empty")
	}
	if c.App.Port == "" {
		return errors.New("APP_PORT must not be empty")
	}
	if len(c.Kafka.Brokers) == 0 {
		return errors.New("KAFKA_BROKERS must list at least one broker")
	}
	return nil
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

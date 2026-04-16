package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	RabbitMQHost string
	RabbitMQPort string
	AdminID      string
	AdminPW      string
	ServerPort   string
}

func Load() (*Config, error) {
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	cfg := &Config{
		RabbitMQHost: os.Getenv("RABBITMQ_HOST"),
		RabbitMQPort: os.Getenv("RABBITMQ_PORT"),
		AdminID:      os.Getenv("ADMIN_ID"),
		AdminPW:      os.Getenv("ADMIN_PW"),
		ServerPort:   serverPort,
	}

	var missing []string
	if cfg.RabbitMQHost == "" {
		missing = append(missing, "RABBITMQ_HOST")
	}
	if cfg.RabbitMQPort == "" {
		missing = append(missing, "RABBITMQ_PORT")
	}
	if cfg.AdminID == "" {
		missing = append(missing, "ADMIN_ID")
	}
	if cfg.AdminPW == "" {
		missing = append(missing, "ADMIN_PW")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

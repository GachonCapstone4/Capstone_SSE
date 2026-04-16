package config

import "os"

type Config struct {
	RabbitMQHost string
	RabbitMQPort string
	ServerPort   string
	AdminID      string
	AdminPW      string
}

func Load() *Config {
	return &Config{
		RabbitMQHost: getEnv("RABBITMQ_HOST", "192.168.2.20"),
		RabbitMQPort: getEnv("RABBITMQ_PORT", "30672"),
		ServerPort:   getEnv("SERVER_PORT", "8080"),
		AdminID:      getEnv("ADMIN_ID", "admin"),
		AdminPW:      getEnv("ADMIN_PW", "admin1234!"),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

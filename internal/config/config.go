package config

import (
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port        string `envconfig:"PORT" default:"8080"`
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`
	BaseURL     string `envconfig:"BASE_URL" default:"http://localhost:8080"`
}

func Load() (Config, error) {
	_ = godotenv.Load() // .env is optional; ignore if absent

	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

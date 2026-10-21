package config

import (
	"errors"
	"os"
)

type Config struct {
	Port        string
	DatabaseURL string
}

func Load() (Config, error) {
	cfg := Config{
		Port:        os.Getenv("PORT"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is not set")
	}
	return cfg, nil
}

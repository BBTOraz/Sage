package app

import (
	"bilge-lib/core"
	"errors"

	"github.com/joho/godotenv"
	"go-simpler.org/env"
)

func LoadEnvConfig() (cfg core.EnvConfig, err error) {
	err = godotenv.Load()
	if err != nil {
		return cfg, errors.New(".env file doesn't exist")
	}
	err = env.Load(&cfg, nil)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

package core

import (
	"errors"
	"strings"
)

type EnvConfig struct {
	Model         string `env:"MODEL"`
	OpenRouterKey string `env:"OPENROUTER_API_KEY"`
	BaseURL       string `env:"BASE_URL"`
	WorkspaceRoot string `env:"WORKSPACE_ROOT"`
}

func (c EnvConfig) Validate() error {
	if strings.TrimSpace(c.Model) == "" {
		return errors.New("MODEL is required")
	}
	if strings.TrimSpace(c.OpenRouterKey) == "" {
		return errors.New("OPENROUTER_API_KEY is required")
	}
	return nil
}

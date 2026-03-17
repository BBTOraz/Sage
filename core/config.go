package core

type EnvConfig struct {
	Model         string `env:"MODEL"`
	OpenRouterKey string `env:"OPENROUTER_API_KEY"`
	BaseURL       string `env:"BASE_URL"`
}

package app

import (
	"bilge-lib/core"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"go-simpler.org/env"
)

func LoadEnvConfig() (cfg core.EnvConfig, err error) {
	cwd, _ := os.Getwd()
	exeDir := executableDir()

	if envFile, ok := findEnvFile(cwd, exeDir); ok {
		if err = godotenv.Load(envFile); err != nil {
			return cfg, fmt.Errorf("failed to load .env from %s: %w", envFile, err)
		}
	}

	err = env.Load(&cfg, nil)
	if err != nil {
		return cfg, fmt.Errorf(
			"environment is not configured: set variables in process env or create .env in one of [%s]: %w",
			strings.Join(candidateEnvFiles(cwd, exeDir), ", "),
			err,
		)
	}
	if err = cfg.Validate(); err != nil {
		return cfg, fmt.Errorf(
			"environment is incomplete: set variables in process env or create .env in one of [%s]: %w",
			strings.Join(candidateEnvFiles(cwd, exeDir), ", "),
			err,
		)
	}
	return cfg, nil
}

func executableDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exePath)
}

func findEnvFile(cwd, exeDir string) (string, bool) {
	for _, candidate := range candidateEnvFiles(cwd, exeDir) {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func candidateEnvFiles(cwd, exeDir string) []string {
	seen := make(map[string]struct{}, 2)
	candidates := make([]string, 0, 2)

	for _, dir := range []string{cwd, exeDir} {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		candidate := filepath.Join(dir, ".env")
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		candidates = append(candidates, candidate)
	}

	return candidates
}

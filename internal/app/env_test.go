package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCandidateEnvFilesDeduplicatesPaths(t *testing.T) {
	dir := t.TempDir()

	got := candidateEnvFiles(dir, dir)
	if len(got) != 1 {
		t.Fatalf("candidateEnvFiles() len = %d, want 1", len(got))
	}
	if want := filepath.Join(dir, ".env"); got[0] != want {
		t.Fatalf("candidateEnvFiles()[0] = %q, want %q", got[0], want)
	}
}

func TestFindEnvFilePrefersCurrentWorkingDirectory(t *testing.T) {
	cwd := t.TempDir()
	exeDir := t.TempDir()

	cwdEnv := filepath.Join(cwd, ".env")
	exeEnv := filepath.Join(exeDir, ".env")
	if err := os.WriteFile(cwdEnv, []byte("MODEL=test\n"), 0o644); err != nil {
		t.Fatalf("write cwd .env: %v", err)
	}
	if err := os.WriteFile(exeEnv, []byte("MODEL=test-exe\n"), 0o644); err != nil {
		t.Fatalf("write exe .env: %v", err)
	}

	got, ok := findEnvFile(cwd, exeDir)
	if !ok {
		t.Fatal("findEnvFile() ok = false, want true")
	}
	if got != cwdEnv {
		t.Fatalf("findEnvFile() = %q, want %q", got, cwdEnv)
	}
}

func TestLoadEnvConfigDoesNotRequireDotEnvWhenProcessEnvIsSet(t *testing.T) {
	t.Setenv("MODEL", "test-model")
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	t.Setenv("BASE_URL", "https://example.com/api")

	cfg, err := LoadEnvConfig()
	if err != nil {
		t.Fatalf("LoadEnvConfig() error = %v", err)
	}
	if cfg.Model != "test-model" {
		t.Fatalf("cfg.Model = %q, want %q", cfg.Model, "test-model")
	}
	if cfg.OpenRouterKey != "test-key" {
		t.Fatalf("cfg.OpenRouterKey = %q, want %q", cfg.OpenRouterKey, "test-key")
	}
	if cfg.BaseURL != "https://example.com/api" {
		t.Fatalf("cfg.BaseURL = %q, want %q", cfg.BaseURL, "https://example.com/api")
	}
}

func TestLoadEnvConfigFailsWhenRequiredEnvMissing(t *testing.T) {
	t.Setenv("MODEL", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("BASE_URL", "")

	_, err := LoadEnvConfig()
	if err == nil {
		t.Fatal("LoadEnvConfig() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "OPENROUTER_API_KEY is required") && !strings.Contains(err.Error(), "MODEL is required") {
		t.Fatalf("LoadEnvConfig() error = %v, want missing env validation", err)
	}
}

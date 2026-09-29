package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
	t.Setenv("PORT", "9090")
	t.Setenv("BASE_URL", "http://localhost:8080")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/db" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://user:pass@localhost:5432/db")
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.BaseURL != "http://localhost:8080" {
		t.Errorf("BaseURL = %q, want default %q", cfg.BaseURL, "http://localhost:8080")
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	os.Unsetenv("DATABASE_URL")
	if _, err := Load(); err == nil {
		t.Fatal("Load() expected error when DATABASE_URL is unset, got nil")
	}
}

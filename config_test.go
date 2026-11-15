package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Тест: config() читает WIKI_HOST/WIKI_TOKEN из конфиг-файла, когда в
// окружении их нет (случай systemd без EnvironmentFile / ручной запуск).
func TestConfigFromFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "wiki-calendar.env")
	content := "# comment\nWIKI_HOST = http://127.0.0.1:8888\n\nWIKI_TOKEN=secret123\nWIKI_REFRESH_INTERVAL_S=5\nWIKI_VERBOSE=1\n"
	if err := os.WriteFile(envPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Окружение очищено от WIKI_*, конфиг-путь переопределён на временный файл.
	os.Unsetenv("WIKI_HOST")
	os.Unsetenv("WIKI_TOKEN")
	os.Unsetenv("WIKI_REFRESH_INTERVAL_S")
	os.Unsetenv("WIKI_VERBOSE")
	os.Setenv("WIKI_CONFIG_PATHS", envPath)
	defer os.Unsetenv("WIKI_CONFIG_PATHS")

	cfg := config()
	if cfg.Host != "http://127.0.0.1:8888" {
		t.Fatalf("Host = %q, want http://127.0.0.1:8888", cfg.Host)
	}
	if cfg.Token != "secret123" {
		t.Fatalf("Token = %q, want secret123", cfg.Token)
	}
	if cfg.RefreshIntervalS != 5 {
		t.Fatalf("RefreshIntervalS = %d, want 5", cfg.RefreshIntervalS)
	}
	if !cfg.Verbose {
		t.Fatal("Verbose = false, want true")
	}
}

// Приоритет: переменная окружения важнее файла.
func TestConfigEnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, "wiki-calendar.env")
	os.WriteFile(envPath, []byte("WIKI_HOST=http://file.example.com\nWIKI_TOKEN=filetoken\n"), 0o644)

	os.Setenv("WIKI_HOST", "http://env.example.com")
	os.Setenv("WIKI_TOKEN", "envtoken")
	os.Setenv("WIKI_CONFIG_PATHS", envPath)
	defer os.Unsetenv("WIKI_HOST")
	defer os.Unsetenv("WIKI_TOKEN")
	defer os.Unsetenv("WIKI_CONFIG_PATHS")

	cfg := config()
	if cfg.Host != "http://env.example.com" || cfg.Token != "envtoken" {
		t.Fatalf("ENV should win: Host=%q Token=%q", cfg.Host, cfg.Token)
	}
}
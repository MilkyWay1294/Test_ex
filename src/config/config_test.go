package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	os.Unsetenv("PORT")
	os.Unsetenv("DB_DSN")
	os.Unsetenv("MYSQL_DSN")
	os.Unsetenv("REDIS_ADDR")
	os.Unsetenv("JWT_SECRET")

	cfg := LoadConfig()

	if cfg.Port != "8080" {
		t.Errorf("Expected default Port to be 8080, got %s", cfg.Port)
	}
	if cfg.RedisAddr != "redis:6379" {
		t.Errorf("Expected default RedisAddr to be redis:6379, got %s", cfg.RedisAddr)
	}
	if cfg.JWTSecret != "super-secret-key" {
		t.Errorf("Expected default JWTSecret to be super-secret-key, got %s", cfg.JWTSecret)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("DB_DSN", "custom-dsn-string")
	os.Setenv("REDIS_ADDR", "localhost:6379")
	os.Setenv("JWT_SECRET", "secret123")

	defer func() {
		os.Unsetenv("PORT")
		os.Unsetenv("DB_DSN")
		os.Unsetenv("REDIS_ADDR")
		os.Unsetenv("JWT_SECRET")
	}()

	cfg := LoadConfig()

	if cfg.Port != "9090" {
		t.Errorf("Expected Port to be overridden to 9090, got %s", cfg.Port)
	}
	if cfg.MySQLDSN != "custom-dsn-string" {
		t.Errorf("Expected MySQLDSN to be overridden to custom-dsn-string, got %s", cfg.MySQLDSN)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("Expected RedisAddr to be overridden to localhost:6379, got %s", cfg.RedisAddr)
	}
	if cfg.JWTSecret != "secret123" {
		t.Errorf("Expected JWTSecret to be overridden to secret123, got %s", cfg.JWTSecret)
	}
}

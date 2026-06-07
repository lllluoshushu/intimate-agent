package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds all configuration for the agent runtime.
type Config struct {
	// LLM API configuration
	LLMBaseURL string
	LLMAPIKey  string
	LLMModel   string

	// Server configuration
	ServerAddr string

	// Database path
	DBPath string
}

// Load reads configuration from .env file first, then environment variables.
// Priority: environment variable > .env file > default value.
func Load() *Config {
	// Try to load .env file from the executable's directory or working directory.
	loadDotEnv()

	return &Config{
		LLMBaseURL: getEnv("LLM_BASE_URL", ""),
		LLMAPIKey:  getEnv("LLM_API_KEY", ""),
		LLMModel:   getEnv("LLM_MODEL", ""),
		ServerAddr: getEnv("SERVER_ADDR", ":8080"),
		DBPath:     getEnv("DB_PATH", "data/agent.db"),
	}
}

// loadDotEnv reads key=value pairs from a .env file and sets them as
// environment variables (only if not already set).
func loadDotEnv() {
	// Search for .env in common locations.
	candidates := []string{".env"}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".env"))
	}

	for _, path := range candidates {
		if err := readEnvFile(path); err == nil {
			return // loaded successfully
		}
	}
}

func readEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments.
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := parseEnvLine(line)
		if !ok {
			continue
		}

		// Only set if not already in environment (env vars take precedence).
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

func parseEnvLine(line string) (key, value string, ok bool) {
	// Support KEY=VALUE and KEY="VALUE" formats.
	idx := strings.Index(line, "=")
	if idx < 0 {
		return "", "", false
	}

	key = strings.TrimSpace(line[:idx])
	value = strings.TrimSpace(line[idx+1:])

	// Remove surrounding quotes.
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
		}
	}

	if key == "" {
		return "", "", false
	}

	return key, value, true
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// String returns a human-readable config summary (masks sensitive fields).
func (c *Config) String() string {
	masked := c.LLMAPIKey
	if len(masked) > 8 {
		masked = masked[:4] + "****" + masked[len(masked)-4:]
	}
	return fmt.Sprintf("LLMBaseURL=%s LLMModel=%s ServerAddr=%s DBPath=%s",
		c.LLMBaseURL, c.LLMModel, c.ServerAddr, c.DBPath)
}

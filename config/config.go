package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultYAMLConfigFile = "config.yaml"
	defaultJSONConfigFile = "config.json"
)

var defaultConfigFiles = []string{defaultYAMLConfigFile, defaultJSONConfigFile}

type Config struct {
	ServerPort string `json:"server_port" yaml:"server_port"`
	TargetURL  string `json:"target_url" yaml:"target_url"`
	LogDir     string `json:"log_dir" yaml:"log_dir"`
	DBPath     string `json:"db_path" yaml:"db_path"`
	UseDB      bool   `json:"use_db" yaml:"use_db"`
}

func Load() (*Config, error) {
	config := &Config{}

	// Try to load from config file. By default, prefer config.yaml and
	// fall back to config.json. CONFIG_FILE still overrides this search.
	if err := config.loadFromFile(); err != nil {
		return nil, err
	}

	// Override with environment variables if they exist
	config.loadFromEnv()

	return config, nil
}

func (c *Config) loadFromFile() error {
	configFile, err := resolveConfigFile()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("read config file %q: %w", configFile, err)
	}

	// Determine file type based on extension
	ext := strings.ToLower(filepath.Ext(configFile))

	switch ext {
	case ".json":
		if err := c.loadFromJSON(data); err != nil {
			return fmt.Errorf("parse JSON config file %q: %w", configFile, err)
		}
	case ".yaml", ".yml":
		if err := c.loadFromYAML(data); err != nil {
			return fmt.Errorf("parse YAML config file %q: %w", configFile, err)
		}
	default:
		// Try JSON by default
		if err := c.loadFromJSON(data); err != nil {
			return fmt.Errorf("parse config file %q as JSON: %w", configFile, err)
		}
	}

	c.setDefaults()
	return nil
}

func resolveConfigFile() (string, error) {
	if configFile := os.Getenv("CONFIG_FILE"); configFile != "" {
		if _, err := os.Stat(configFile); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("config file %q not found", configFile)
			}
			return "", fmt.Errorf("cannot access config file %q: %w", configFile, err)
		}
		return configFile, nil
	}

	for _, configFile := range defaultConfigFiles {
		if _, err := os.Stat(configFile); err == nil {
			return configFile, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("cannot access config file %q: %w", configFile, err)
		}
	}

	return "", fmt.Errorf("no config file found: tried %s", strings.Join(defaultConfigFiles, ", "))
}

func (c *Config) loadFromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

func (c *Config) loadFromYAML(data []byte) error {
	// Simple YAML parsing without external dependencies
	// This is a basic implementation that handles simple key-value pairs
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)

		switch key {
		case "server_port":
			c.ServerPort = value
		case "target_url":
			c.TargetURL = value
		case "log_dir":
			c.LogDir = value
		case "db_path":
			c.DBPath = value
		case "use_db":
			c.UseDB = (value == "true" || value == "1")
		}
	}
	return nil
}

func (c *Config) setDefaults() {
	if c.ServerPort == "" {
		c.ServerPort = ":8080"
	}
	if c.TargetURL == "" {
		c.TargetURL = "https://api.aicodewith.com"
	}
	if c.LogDir == "" {
		c.LogDir = "./logs"
	}
	if c.DBPath == "" {
		c.DBPath = "./logs/proxy.db"
	}

	// Ensure log directory uses proper path separator for relative paths
	// but preserves absolute paths as-is (including Unix-style on Windows)
	if !isAbsolutePath(c.LogDir) {
		c.LogDir = filepath.Join(".", c.LogDir)
	}

	// Handle DB path similarly
	if !isAbsolutePath(c.DBPath) {
		dbDir := filepath.Dir(c.DBPath)
		if !isAbsolutePath(dbDir) {
			dbDir = filepath.Join(".", dbDir)
			c.DBPath = filepath.Join(dbDir, filepath.Base(c.DBPath))
		}
	}
}

func (c *Config) loadFromEnv() {
	if port := os.Getenv("SERVER_PORT"); port != "" {
		c.ServerPort = port
	}
	if url := os.Getenv("TARGET_URL"); url != "" {
		c.TargetURL = url
	}
	if dir := os.Getenv("LOG_DIR"); dir != "" {
		c.LogDir = dir
		// Only apply filepath.Join if it's not already absolute
		// This preserves Unix-style paths on Windows for testing
		if !isAbsolutePath(c.LogDir) {
			c.LogDir = filepath.Join(".", c.LogDir)
		}
	}
	if dbPath := os.Getenv("DB_PATH"); dbPath != "" {
		c.DBPath = dbPath
		// Only apply filepath.Join if it's not already absolute
		// This preserves Unix-style paths on Windows for testing
		if !isAbsolutePath(c.DBPath) {
			c.DBPath = filepath.Join(".", c.DBPath)
		}
	}
	if useDB := strings.TrimSpace(os.Getenv("USE_DB")); useDB != "" {
		switch strings.ToLower(useDB) {
		case "true", "1":
			c.UseDB = true
		case "false", "0":
			c.UseDB = false
		}
	}
}

// isAbsolutePath checks if a path is absolute.
// It handles both Unix-style (/) and Windows-style paths.
func isAbsolutePath(path string) bool {
	// Check for Unix-style absolute path
	if strings.HasPrefix(path, "/") {
		return true
	}
	// Use filepath.IsAbs for native OS checking
	return filepath.IsAbs(path)
}

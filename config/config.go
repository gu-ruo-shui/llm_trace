package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ServerPort string `json:"server_port" yaml:"server_port"`
	TargetURL  string `json:"target_url" yaml:"target_url"`
	LogDir     string `json:"log_dir" yaml:"log_dir"`
	DBPath     string `json:"db_path" yaml:"db_path"`
	UseDB      bool   `json:"use_db" yaml:"use_db"`
}

func Load() *Config {
	config := &Config{}
	
	// Try to load from config file
	config.loadFromFile()
	
	// Override with environment variables if they exist
	config.loadFromEnv()
	
	return config
}

func (c *Config) loadFromFile() {
	configFile := getEnv("CONFIG_FILE", "config.json")
	
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Config file doesn't exist, use defaults
		c.setDefaults()
		return
	}
	
	data, err := os.ReadFile(configFile)
	if err != nil {
		// Use defaults if can't read file
		c.setDefaults()
		return
	}
	
	// Determine file type based on extension
	ext := strings.ToLower(filepath.Ext(configFile))
	
	switch ext {
	case ".json":
		c.loadFromJSON(data)
	case ".yaml", ".yml":
		c.loadFromYAML(data)
	default:
		// Try JSON by default
		c.loadFromJSON(data)
	}
}

func (c *Config) loadFromJSON(data []byte) {
	err := json.Unmarshal(data, c)
	if err != nil {
		c.setDefaults()
		return
	}
	c.setDefaults()
}

func (c *Config) loadFromYAML(data []byte) {
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
	c.setDefaults()
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
	if useDB := os.Getenv("USE_DB"); useDB == "true" || useDB == "1" {
		c.UseDB = true
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
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

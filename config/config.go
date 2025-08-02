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
	
	// Ensure log directory is absolute path
	if !filepath.IsAbs(c.LogDir) {
		c.LogDir = filepath.Join(".", c.LogDir)
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
		if !filepath.IsAbs(c.LogDir) {
			c.LogDir = filepath.Join(".", c.LogDir)
		}
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

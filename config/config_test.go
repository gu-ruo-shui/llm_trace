package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultConfig(t *testing.T) {
	// 确保没有配置文件存在
	os.Remove("config.json")
	os.Remove("config.yaml")
	
	config := Load()
	
	if config.ServerPort != ":8080" {
		t.Errorf("Expected default ServerPort :8080, got %s", config.ServerPort)
	}
	if config.TargetURL != "https://api.aicodewith.com" {
		t.Errorf("Expected default TargetURL https://api.aicodewith.com, got %s", config.TargetURL)
	}
	if config.LogDir != filepath.Join(".", "logs") {
		t.Errorf("Expected default LogDir ./logs, got %s", config.LogDir)
	}
	if config.DBPath != filepath.Join(".", "logs", "proxy.db") {
		t.Errorf("Expected default DBPath ./logs/proxy.db, got %s", config.DBPath)
	}
	if config.UseDB != false {
		t.Errorf("Expected default UseDB false, got %t", config.UseDB)
	}
}

func TestLoad_JSONConfig(t *testing.T) {
	configContent := `{
		"server_port": ":9090",
		"target_url": "https://api.openai.com",
		"log_dir": "./test_logs",
		"db_path": "./test_logs/test.db",
		"use_db": true
	}`
	
	// 创建临时配置文件
	tempFile := "temp_config.json"
	os.WriteFile(tempFile, []byte(configContent), 0644)
	defer os.Remove(tempFile)
	
	// 设置环境变量指向临时文件
	os.Setenv("CONFIG_FILE", tempFile)
	defer os.Unsetenv("CONFIG_FILE")
	
	config := Load()
	
	if config.ServerPort != ":9090" {
		t.Errorf("Expected ServerPort :9090, got %s", config.ServerPort)
	}
	if config.TargetURL != "https://api.openai.com" {
		t.Errorf("Expected TargetURL https://api.openai.com, got %s", config.TargetURL)
	}
	if config.LogDir != filepath.Join(".", "test_logs") {
		t.Errorf("Expected LogDir ./test_logs, got %s", config.LogDir)
	}
	if config.DBPath != filepath.Join(".", "test_logs", "test.db") {
		t.Errorf("Expected DBPath ./test_logs/test.db, got %s", config.DBPath)
	}
	if config.UseDB != true {
		t.Errorf("Expected UseDB true, got %t", config.UseDB)
	}
}

func TestLoad_YAMLConfig(t *testing.T) {
	configContent := `server_port: ":7070"
target_url: "https://api.anthropic.com"
log_dir: "./yaml_logs"
db_path: "./yaml_logs/yaml.db"
use_db: true`
	
	// 创建临时YAML配置文件
	tempFile := "temp_config.yaml"
	os.WriteFile(tempFile, []byte(configContent), 0644)
	defer os.Remove(tempFile)
	
	// 设置环境变量指向临时文件
	os.Setenv("CONFIG_FILE", tempFile)
	defer os.Unsetenv("CONFIG_FILE")
	
	config := Load()
	
	if config.ServerPort != ":7070" {
		t.Errorf("Expected ServerPort :7070, got %s", config.ServerPort)
	}
	if config.TargetURL != "https://api.anthropic.com" {
		t.Errorf("Expected TargetURL https://api.anthropic.com, got %s", config.TargetURL)
	}
	if config.LogDir != filepath.Join(".", "yaml_logs") {
		t.Errorf("Expected LogDir ./yaml_logs, got %s", config.LogDir)
	}
	if config.DBPath != filepath.Join(".", "yaml_logs", "yaml.db") {
		t.Errorf("Expected DBPath ./yaml_logs/yaml.db, got %s", config.DBPath)
	}
	if config.UseDB != true {
		t.Errorf("Expected UseDB true, got %t", config.UseDB)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	// 清理环境变量
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("TARGET_URL")
	os.Unsetenv("LOG_DIR")
	os.Unsetenv("DB_PATH")
	os.Unsetenv("USE_DB")
	
	// 设置环境变量
	os.Setenv("SERVER_PORT", ":9999")
	os.Setenv("TARGET_URL", "https://api.test.com")
	os.Setenv("LOG_DIR", "/tmp/logs")
	os.Setenv("DB_PATH", "/tmp/test.db")
	os.Setenv("USE_DB", "true")
	
	defer func() {
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("TARGET_URL")
		os.Unsetenv("LOG_DIR")
		os.Unsetenv("DB_PATH")
		os.Unsetenv("USE_DB")
	}()
	
	config := Load()
	
	if config.ServerPort != ":9999" {
		t.Errorf("Expected ServerPort :9999 from env, got %s", config.ServerPort)
	}
	if config.TargetURL != "https://api.test.com" {
		t.Errorf("Expected TargetURL https://api.test.com from env, got %s", config.TargetURL)
	}
	if config.LogDir != "/tmp/logs" {
		t.Errorf("Expected LogDir /tmp/logs from env, got %s", config.LogDir)
	}
	if config.DBPath != "/tmp/test.db" {
		t.Errorf("Expected DBPath /tmp/test.db from env, got %s", config.DBPath)
	}
	if config.UseDB != true {
		t.Errorf("Expected UseDB true from env, got %t", config.UseDB)
	}
}

func TestLoad_BadJSONConfig(t *testing.T) {
	configContent := `{
		"server_port": ":9090",
		"target_url": "https://api.openai.com",
		"invalid_json:`
	
	// 创建临时配置文件
	tempFile := "temp_bad_config.json"
	os.WriteFile(tempFile, []byte(configContent), 0644)
	defer os.Remove(tempFile)
	
	// 设置环境变量指向临时文件
	os.Setenv("CONFIG_FILE", tempFile)
	defer os.Unsetenv("CONFIG_FILE")
	
	config := Load()
	
	// 应该使用默认值，而不是panic
	if config.ServerPort != ":8080" {
		t.Errorf("Expected fallback to default ServerPort :8080, got %s", config.ServerPort)
	}
}

func TestLoad_AbsolutePaths(t *testing.T) {
	configContent := `{
		"log_dir": "/absolute/logs",
		"db_path": "/absolute/path/test.db"
	}`
	
	tempFile := "temp_abs_config.json"
	os.WriteFile(tempFile, []byte(configContent), 0644)
	defer os.Remove(tempFile)
	
	os.Setenv("CONFIG_FILE", tempFile)
	defer os.Unsetenv("CONFIG_FILE")
	
	config := Load()
	
	if config.LogDir != "/absolute/logs" {
		t.Errorf("Expected absolute LogDir /absolute/logs, got %s", config.LogDir)
	}
	if config.DBPath != "/absolute/path/test.db" {
		t.Errorf("Expected absolute DBPath /absolute/path/test.db, got %s", config.DBPath)
	}
}
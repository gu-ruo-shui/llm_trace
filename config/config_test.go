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

func TestLoadFromFile_ReadError(t *testing.T) {
	// Create a directory with same name as config file to cause read error
	tempDir := "temp_config_dir.json"
	os.Mkdir(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	os.Setenv("CONFIG_FILE", tempDir)
	defer os.Unsetenv("CONFIG_FILE")

	config := Load()

	// Should fall back to defaults when read fails
	if config.ServerPort != ":8080" {
		t.Errorf("Expected default ServerPort :8080, got %s", config.ServerPort)
	}
}

func TestLoadFromFile_UnknownExtension(t *testing.T) {
	configContent := `{
		"server_port": ":9191",
		"target_url": "https://api.test.com"
	}`

	// Create file with unknown extension
	tempFile := "temp_config.txt"
	os.WriteFile(tempFile, []byte(configContent), 0644)
	defer os.Remove(tempFile)

	os.Setenv("CONFIG_FILE", tempFile)
	defer os.Unsetenv("CONFIG_FILE")

	config := Load()

	// Should try to parse as JSON by default
	if config.ServerPort != ":9191" {
		t.Errorf("Expected ServerPort :9191, got %s", config.ServerPort)
	}
}

func TestLoadFromYAML_WithCommentsAndEmptyLines(t *testing.T) {
	configContent := `# This is a comment
server_port: ":8181"

# Another comment
target_url: "https://api.example.com"

log_dir: "./custom_logs"
# db_path is commented out
# db_path: "./custom.db"

use_db: true
`

	tempFile := "temp_config_comments.yaml"
	os.WriteFile(tempFile, []byte(configContent), 0644)
	defer os.Remove(tempFile)

	os.Setenv("CONFIG_FILE", tempFile)
	defer os.Unsetenv("CONFIG_FILE")

	config := Load()

	if config.ServerPort != ":8181" {
		t.Errorf("Expected ServerPort :8181, got %s", config.ServerPort)
	}
	if config.LogDir != filepath.Join(".", "custom_logs") {
		t.Errorf("Expected LogDir ./custom_logs, got %s", config.LogDir)
	}
	// db_path should use default since it's commented out
	if config.DBPath != filepath.Join(".", "logs", "proxy.db") {
		t.Errorf("Expected default DBPath, got %s", config.DBPath)
	}
}

func TestLoadFromYAML_InvalidLines(t *testing.T) {
	configContent := `server_port: ":8282"
this is not a valid yaml line
target_url: "https://api.test.com"
another invalid line without colon
log_dir: "./test_logs"
key_without_value:
use_db: 1`

	tempFile := "temp_config_invalid.yaml"
	os.WriteFile(tempFile, []byte(configContent), 0644)
	defer os.Remove(tempFile)

	os.Setenv("CONFIG_FILE", tempFile)
	defer os.Unsetenv("CONFIG_FILE")

	config := Load()

	// Valid lines should still be parsed
	if config.ServerPort != ":8282" {
		t.Errorf("Expected ServerPort :8282, got %s", config.ServerPort)
	}
	if config.TargetURL != "https://api.test.com" {
		t.Errorf("Expected TargetURL https://api.test.com, got %s", config.TargetURL)
	}
	if config.UseDB != true {
		t.Errorf("Expected UseDB true (from '1'), got %t", config.UseDB)
	}
}

func TestLoadFromEnv_RelativePaths(t *testing.T) {
	// Clear any existing env vars
	os.Unsetenv("LOG_DIR")
	os.Unsetenv("DB_PATH")

	// Set relative paths in environment
	os.Setenv("LOG_DIR", "relative/logs")
	os.Setenv("DB_PATH", "relative/db/proxy.db")

	defer func() {
		os.Unsetenv("LOG_DIR")
		os.Unsetenv("DB_PATH")
	}()

	config := Load()

	// Should apply filepath.Join for relative paths
	expectedLogDir := filepath.Join(".", "relative", "logs")
	expectedDBPath := filepath.Join(".", "relative", "db", "proxy.db")

	if config.LogDir != expectedLogDir {
		t.Errorf("Expected LogDir %s, got %s", expectedLogDir, config.LogDir)
	}
	if config.DBPath != expectedDBPath {
		t.Errorf("Expected DBPath %s, got %s", expectedDBPath, config.DBPath)
	}
}

func TestLoadFromEnv_UseDBVariations(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{"true lowercase", "true", true},
		{"1", "1", true},
		{"false", "false", false},
		{"0", "0", false},
		{"empty", "", false},
		{"True uppercase", "True", false},
		{"yes", "yes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("USE_DB", tt.value)
			defer os.Unsetenv("USE_DB")

			config := Load()

			if config.UseDB != tt.expected {
				t.Errorf("For USE_DB=%s, expected %t, got %t", tt.value, tt.expected, config.UseDB)
			}
		})
	}
}

func TestLoadFromYAML_QuotedValues(t *testing.T) {
	configContent := `server_port: ":9393"
target_url: 'https://api.quoted.com'
log_dir: "./quoted_logs"
db_path: "./quoted.db"
use_db: "true"`

	tempFile := "temp_config_quoted.yaml"
	os.WriteFile(tempFile, []byte(configContent), 0644)
	defer os.Remove(tempFile)

	os.Setenv("CONFIG_FILE", tempFile)
	defer os.Unsetenv("CONFIG_FILE")

	config := Load()

	// Quotes should be stripped
	if config.ServerPort != ":9393" {
		t.Errorf("Expected ServerPort :9393, got %s", config.ServerPort)
	}
	if config.TargetURL != "https://api.quoted.com" {
		t.Errorf("Expected TargetURL without quotes, got %s", config.TargetURL)
	}
	if config.UseDB != true {
		t.Errorf("Expected UseDB true from quoted 'true', got %t", config.UseDB)
	}
}

func TestIsAbsolutePath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"Unix absolute", "/usr/local/bin", true},
		{"Unix home", "/home/user", true},
		{"Unix root", "/", true},
		{"Relative dot", "./logs", false},
		{"Relative no dot", "logs", false},
		{"Relative parent", "../logs", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAbsolutePath(tt.path)
			if result != tt.expected {
				t.Errorf("isAbsolutePath(%s) = %t, want %t", tt.path, result, tt.expected)
			}
		})
	}
}

func TestSetDefaults_WithPartialConfig(t *testing.T) {
	// Test that setDefaults only sets missing values
	config := &Config{
		ServerPort: ":9494",
		// Leave other fields empty
	}

	config.setDefaults()

	// ServerPort should remain unchanged
	if config.ServerPort != ":9494" {
		t.Errorf("ServerPort should not be changed, got %s", config.ServerPort)
	}

	// Other fields should get defaults
	if config.TargetURL != "https://api.aicodewith.com" {
		t.Errorf("Expected default TargetURL, got %s", config.TargetURL)
	}
	if config.LogDir != filepath.Join(".", "logs") {
		t.Errorf("Expected default LogDir, got %s", config.LogDir)
	}
}

func TestLoadFromYAML_MixedFormats(t *testing.T) {
	configContent := `# Config with various formatting
server_port:":5050"
target_url:   https://api.nospace.com   
log_dir: ./logs_with_spaces  
  db_path: ./indented.db
use_db:1`

	tempFile := "temp_config_mixed.yaml"
	os.WriteFile(tempFile, []byte(configContent), 0644)
	defer os.Remove(tempFile)

	os.Setenv("CONFIG_FILE", tempFile)
	defer os.Unsetenv("CONFIG_FILE")

	config := Load()

	if config.ServerPort != ":5050" {
		t.Errorf("Expected ServerPort :5050, got %s", config.ServerPort)
	}
	if config.TargetURL != "https://api.nospace.com" {
		t.Errorf("Expected trimmed TargetURL, got %s", config.TargetURL)
	}
	if config.LogDir != filepath.Join(".", "logs_with_spaces") {
		t.Errorf("Expected trimmed LogDir, got %s", config.LogDir)
	}
	if config.DBPath != filepath.Join(".", "indented.db") {
		t.Errorf("Expected DBPath from indented line, got %s", config.DBPath)
	}
}

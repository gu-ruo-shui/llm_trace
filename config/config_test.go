package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func chdir(t *testing.T, dir string) {
	t.Helper()

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Failed to change directory to %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatalf("Failed to restore current directory to %s: %v", oldDir, err)
		}
	})
}

func mustLoad(t *testing.T) *Config {
	t.Helper()

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	return config
}

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file %s: %v", path, err)
	}
}

func tempConfigFile(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	writeConfigFile(t, path, content)
	return path
}

func TestLoad_DefaultPriorityUsesYAMLThenJSON(t *testing.T) {
	t.Setenv("CONFIG_FILE", "")
	chdir(t, t.TempDir())

	writeConfigFile(t, "config.yaml", `target_url: "https://yaml.example.com"`)
	writeConfigFile(t, "config.json", `{"target_url":"https://json.example.com"}`)

	config := mustLoad(t)
	if config.TargetURL != "https://yaml.example.com" {
		t.Errorf("Expected config.yaml to be preferred, got %s", config.TargetURL)
	}

	if err := os.Remove("config.yaml"); err != nil {
		t.Fatalf("Failed to remove config.yaml: %v", err)
	}

	config = mustLoad(t)
	if config.TargetURL != "https://json.example.com" {
		t.Errorf("Expected config.json fallback, got %s", config.TargetURL)
	}

	if err := os.Remove("config.json"); err != nil {
		t.Fatalf("Failed to remove config.json: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Expected missing config files to return an error")
	}
	if !strings.Contains(err.Error(), "no config file found") {
		t.Fatalf("Expected missing config error, got %v", err)
	}
}

func TestLoad_CONFIGFILEOverridesDefaultSearch(t *testing.T) {
	chdir(t, t.TempDir())
	writeConfigFile(t, "config.yaml", `target_url: "https://yaml.example.com"`)
	customJSON := filepath.Join(t.TempDir(), "custom.json")
	writeConfigFile(t, customJSON, `{"target_url":"https://custom.example.com"}`)
	t.Setenv("CONFIG_FILE", customJSON)

	config := mustLoad(t)
	if config.TargetURL != "https://custom.example.com" {
		t.Errorf("Expected CONFIG_FILE to override default search, got %s", config.TargetURL)
	}
}

func TestLoad_CONFIGFILEMissingReportsError(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))

	_, err := Load()
	if err == nil {
		t.Fatal("Expected missing CONFIG_FILE to return an error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("Expected not found error, got %v", err)
	}
}

func TestLoad_DefaultConfigValuesFromEmptyYAML(t *testing.T) {
	t.Setenv("CONFIG_FILE", tempConfigFile(t, "empty.yaml", ""))

	config := mustLoad(t)

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

	t.Setenv("CONFIG_FILE", tempConfigFile(t, "temp_config.json", configContent))

	config := mustLoad(t)

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

	t.Setenv("CONFIG_FILE", tempConfigFile(t, "temp_config.yaml", configContent))

	config := mustLoad(t)

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
	t.Setenv("CONFIG_FILE", tempConfigFile(t, "base.yaml", ""))
	t.Setenv("SERVER_PORT", ":9999")
	t.Setenv("TARGET_URL", "https://api.test.com")
	t.Setenv("LOG_DIR", "/tmp/logs")
	t.Setenv("DB_PATH", "/tmp/test.db")
	t.Setenv("USE_DB", "true")

	config := mustLoad(t)

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

	t.Setenv("CONFIG_FILE", tempConfigFile(t, "temp_bad_config.json", configContent))

	_, err := Load()
	if err == nil {
		t.Fatal("Expected invalid JSON config to return an error")
	}
	if !strings.Contains(err.Error(), "parse JSON") {
		t.Fatalf("Expected parse JSON error, got %v", err)
	}
}

func TestLoad_AbsolutePaths(t *testing.T) {
	configContent := `{
		"log_dir": "/absolute/logs",
		"db_path": "/absolute/path/test.db"
	}`

	t.Setenv("CONFIG_FILE", tempConfigFile(t, "temp_abs_config.json", configContent))

	config := mustLoad(t)

	if config.LogDir != "/absolute/logs" {
		t.Errorf("Expected absolute LogDir /absolute/logs, got %s", config.LogDir)
	}
	if config.DBPath != "/absolute/path/test.db" {
		t.Errorf("Expected absolute DBPath /absolute/path/test.db, got %s", config.DBPath)
	}
}

func TestLoadFromFile_ReadError(t *testing.T) {
	// Create a directory with same name as config file to cause read error.
	tempDir := filepath.Join(t.TempDir(), "temp_config_dir.json")
	if err := os.Mkdir(tempDir, 0755); err != nil {
		t.Fatalf("Failed to create temp config dir: %v", err)
	}
	t.Setenv("CONFIG_FILE", tempDir)

	_, err := Load()
	if err == nil {
		t.Fatal("Expected read error when CONFIG_FILE points to a directory")
	}
	if !strings.Contains(err.Error(), "read config file") {
		t.Fatalf("Expected read config file error, got %v", err)
	}
}

func TestLoadFromFile_UnknownExtension(t *testing.T) {
	configContent := `{
		"server_port": ":9191",
		"target_url": "https://api.test.com"
	}`

	t.Setenv("CONFIG_FILE", tempConfigFile(t, "temp_config.txt", configContent))

	config := mustLoad(t)

	// Should try to parse as JSON by default.
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

	t.Setenv("CONFIG_FILE", tempConfigFile(t, "temp_config_comments.yaml", configContent))

	config := mustLoad(t)

	if config.ServerPort != ":8181" {
		t.Errorf("Expected ServerPort :8181, got %s", config.ServerPort)
	}
	if config.LogDir != filepath.Join(".", "custom_logs") {
		t.Errorf("Expected LogDir ./custom_logs, got %s", config.LogDir)
	}
	// db_path should use default since it's commented out.
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

	t.Setenv("CONFIG_FILE", tempConfigFile(t, "temp_config_invalid.yaml", configContent))

	config := mustLoad(t)

	// Valid lines should still be parsed.
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
	t.Setenv("CONFIG_FILE", tempConfigFile(t, "base.yaml", ""))
	t.Setenv("LOG_DIR", "relative/logs")
	t.Setenv("DB_PATH", "relative/db/proxy.db")

	config := mustLoad(t)

	// Should apply filepath.Join for relative paths.
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
	t.Setenv("CONFIG_FILE", tempConfigFile(t, "base.yaml", ""))

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
		{"True uppercase", "True", true},
		{"FALSE uppercase", "FALSE", false},
		{"yes", "yes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("USE_DB", tt.value)

			config := mustLoad(t)

			if config.UseDB != tt.expected {
				t.Errorf("For USE_DB=%s, expected %t, got %t", tt.value, tt.expected, config.UseDB)
			}
		})
	}
}

func TestLoadFromEnv_UseDBCanDisableConfigValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"false", "false"},
		{"False", "False"},
		{"FALSE", "FALSE"},
		{"0", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CONFIG_FILE", tempConfigFile(t, "base.yaml", "use_db: true"))
			t.Setenv("USE_DB", tt.value)

			config := mustLoad(t)

			if config.UseDB {
				t.Errorf("For USE_DB=%s, expected false override, got true", tt.value)
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

	t.Setenv("CONFIG_FILE", tempConfigFile(t, "temp_config_quoted.yaml", configContent))

	config := mustLoad(t)

	// Quotes should be stripped.
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
	// Test that setDefaults only sets missing values.
	config := &Config{
		ServerPort: ":9494",
		// Leave other fields empty.
	}

	config.setDefaults()

	// ServerPort should remain unchanged.
	if config.ServerPort != ":9494" {
		t.Errorf("ServerPort should not be changed, got %s", config.ServerPort)
	}

	// Other fields should get defaults.
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

	t.Setenv("CONFIG_FILE", tempConfigFile(t, "temp_config_mixed.yaml", configContent))

	config := mustLoad(t)

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

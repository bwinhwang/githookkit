package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {

	homeDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	configPath := filepath.Join(homeDir, ".githook_config")
	// Test 1: When config file doesn't exist
	config, err := LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() returned error: %v", err)
	}
	if len(config.ProjectsWhitelist) != 0 {
		t.Errorf("ProjectsWhitelist should be empty when config file doesn't exist")
	}
	if len(config.ProjectSizeLimits) != 0 {
		t.Errorf("ProjectSizeLimits should be empty when config file doesn't exist")
	}
	if config.LogConfig.Level != "" {
		t.Errorf("LogConfig.Level should be empty when config file doesn't exist")
	}
	if config.LogConfig.Output != "" {
		t.Errorf("LogConfig.Output should be empty when config file doesn't exist")
	}

	// Test 2: Valid config file
	validConfig := `
projects_whitelist:
  - project1
  - project2
project_size_limits:
  project1: 10485760
  project2: 20971520
log_config:
  level: debug
  output: /var/log/githook.log
`
	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	config, err = LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() returned error: %v", err)
	}
	if len(config.ProjectsWhitelist) != 2 {
		t.Errorf("ProjectsWhitelist length should be 2, got %d", len(config.ProjectsWhitelist))
	}
	if config.ProjectsWhitelist[0] != "project1" || config.ProjectsWhitelist[1] != "project2" {
		t.Errorf("ProjectsWhitelist content is incorrect")
	}
	if len(config.ProjectSizeLimits) != 2 {
		t.Errorf("ProjectSizeLimits length should be 2, got %d", len(config.ProjectSizeLimits))
	}
	if config.ProjectSizeLimits["project1"] != 10485760 || config.ProjectSizeLimits["project2"] != 20971520 {
		t.Errorf("ProjectSizeLimits content is incorrect")
	}
	// Test log config
	if config.LogConfig.Level != "debug" {
		t.Errorf("LogConfig.Level should be 'debug', got '%s'", config.LogConfig.Level)
	}
	if config.LogConfig.Output != "/var/log/githook.log" {
		t.Errorf("LogConfig.Output should be '/var/log/githook.log', got '%s'", config.LogConfig.Output)
	}

	// Test 3: Invalid config file
	invalidConfig := `invalid yaml content`
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}

	config, err = LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() returned error: %v", err)
	}
	if len(config.ProjectsWhitelist) != 0 {
		t.Errorf("ProjectsWhitelist should be empty for invalid config")
	}
	if len(config.ProjectSizeLimits) != 0 {
		t.Errorf("ProjectSizeLimits should be empty for invalid config")
	}
	// Log config should be empty
	if config.LogConfig.Level != "" {
		t.Errorf("LogConfig.Level should be empty for invalid config")
	}
	if config.LogConfig.Output != "" {
		t.Errorf("LogConfig.Output should be empty for invalid config")
	}

}

func TestIsProjectWhitelisted(t *testing.T) {
	config := Config{
		ProjectsWhitelist: []string{"project1", "project2"},
	}

	tests := []struct {
		project  string
		expected bool
	}{
		{"project1", true},
		{"project2", true},
		{"project3", false},
		{"", false},
	}

	for _, test := range tests {
		result := IsProjectWhitelisted(config, test.project)
		if result != test.expected {
			t.Errorf("IsProjectWhitelisted(%s) = %v, expected %v", test.project, result, test.expected)
		}
	}
}

func TestGetSizeLimit(t *testing.T) {
	oldEnv := os.Getenv("GITHOOK_FILE_SIZE_MAX")
	defer os.Setenv("GITHOOK_FILE_SIZE_MAX", oldEnv)

	config := Config{
		ProjectSizeLimits: map[string]int64{
			"project1": 10 * 1024 * 1024,
			"project2": 20 * 1024 * 1024,
		},
	}

	// Test 1: Use default value
	os.Unsetenv("GITHOOK_FILE_SIZE_MAX")
	result := GetSizeLimit(config, "project3")
	if result != 5*1024*1024 {
		t.Errorf("GetSizeLimit(project3) = %d, expected %d", result, 5*1024*1024)
	}

	// Test 2: Use environment variable
	os.Setenv("GITHOOK_FILE_SIZE_MAX", "15728640") // 15MB
	result = GetSizeLimit(config, "project3")
	if result != 15*1024*1024 {
		t.Errorf("GetSizeLimit(project3) = %d, expected %d", result, 15*1024*1024)
	}

	// Test 3: Use project-specific limit
	result = GetSizeLimit(config, "project1")
	if result != 10*1024*1024 {
		t.Errorf("GetSizeLimit(project1) = %d, expected %d", result, 10*1024*1024)
	}

	result = GetSizeLimit(config, "project2")
	if result != 20*1024*1024 {
		t.Errorf("GetSizeLimit(project2) = %d, expected %d", result, 20*1024*1024)
	}
}

func TestContains(t *testing.T) {
	slice := []string{"a", "b", "c"}

	tests := []struct {
		item     string
		expected bool
	}{
		{"a", true},
		{"b", true},
		{"c", true},
		{"d", false},
		{"", false},
	}

	for _, test := range tests {
		result := Contains(slice, test.item)
		if result != test.expected {
			t.Errorf("Contains(slice, %s) = %v, 期望 %v", test.item, result, test.expected)
		}
	}
}
func TestInitLogger(t *testing.T) {
	oldLevel := os.Getenv("GITHOOK_LOG_LEVEL")
	oldOutput := os.Getenv("GITHOOK_LOG_OUTPUT")
	defer func() {
		os.Setenv("GITHOOK_LOG_LEVEL", oldLevel)
		os.Setenv("GITHOOK_LOG_OUTPUT", oldOutput)
	}()

	tests := []struct {
		name           string
		config         Config
		envLevel       string
		envOutput      string
		expectedLevel  string
		expectedOutput string
		expectError    bool
	}{
		{
			name: "Use config file",
			config: Config{
				LogConfig: LogConfig{
					Level:  "info",
					Output: "/tmp/test.log",
				},
			},
			expectedLevel:  "info",
			expectedOutput: "/tmp/test.log",
		},
		{
			name: "Use environment variables to override",
			config: Config{
				LogConfig: LogConfig{
					Level:  "info",
					Output: "/tmp/test.log",
				},
			},
			envLevel:       "debug",
			envOutput:      "stdout",
			expectedLevel:  "debug",
			expectedOutput: "stdout",
		},
		{
			name:           "Use default values",
			config:         Config{},
			expectedLevel:  "info",
			expectedOutput: "stderr",
		},
		{
			name: "Invalid file path",
			config: Config{
				LogConfig: LogConfig{
					Output: "/invalid/path/test.log",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			os.Setenv("GITHOOK_LOG_LEVEL", tt.envLevel)
			os.Setenv("GITHOOK_LOG_OUTPUT", tt.envOutput)

			logger, err := InitLogger(tt.config)
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("InitLogger() returned error: %v", err)
				return
			}

			// Test log output
			logger.Printf("Test log level: %s, output: %s", tt.expectedLevel, tt.expectedOutput)
		})
	}
}

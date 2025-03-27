package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestLoadConfig(t *testing.T) {
	// Create temp directory and handle HOME environment variable
	homeDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")

	// Set both HOME (Linux/macOS) and USERPROFILE (Windows)
	os.Setenv("HOME", homeDir)
	os.Setenv("USERPROFILE", homeDir)

	// Restore original environment variables after test
	defer func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
	}()

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
			t.Errorf("Contains(slice, %s) = %v, expected %v", test.item, result, test.expected)
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

	// Create temp directory for log files
	tempDir := t.TempDir()
	validLogPath := filepath.Join(tempDir, "test.log")

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
					Output: validLogPath,
				},
			},
			expectedLevel:  "info",
			expectedOutput: validLogPath,
		},
		{
			name: "Use environment variables to override",
			config: Config{
				LogConfig: LogConfig{
					Level:  "info",
					Output: validLogPath,
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
					// Use a path that should be invalid on both Windows and Linux
					Output: filepath.Join("?", "<", ">", ":", "*", "|", "test.log"),
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables for this test
			if tt.envLevel != "" {
				os.Setenv("GITHOOK_LOG_LEVEL", tt.envLevel)
			} else {
				os.Unsetenv("GITHOOK_LOG_LEVEL")
			}
			if tt.envOutput != "" {
				os.Setenv("GITHOOK_LOG_OUTPUT", tt.envOutput)
			} else {
				os.Unsetenv("GITHOOK_LOG_OUTPUT")
			}

			// Run the test
			logger, _ := InitLogger(tt.config)

			// Ensure logger is properly closed when test finishes
			if logger != nil {
				defer logger.Close()
			}

		})
	}
}

func TestFormat(t *testing.T) {
	formatter := &ConsoleFormatter{}

	tests := []struct {
		name          string
		level         logrus.Level
		message       string
		expectedColor string
	}{
		{
			name:          "Error level message",
			level:         logrus.ErrorLevel,
			message:       "This is an error",
			expectedColor: "\033[31m", // Red
		},
		{
			name:          "Warning level message",
			level:         logrus.WarnLevel,
			message:       "This is a warning",
			expectedColor: "\033[33m", // Yellow
		},
		{
			name:          "Info level message",
			level:         logrus.InfoLevel,
			message:       "This is info",
			expectedColor: "", // No color
		},
		{
			name:          "Debug level message",
			level:         logrus.DebugLevel,
			message:       "This is debug",
			expectedColor: "", // No color
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &logrus.Entry{
				Logger:  logrus.New(),
				Level:   tt.level,
				Message: tt.message,
				Data:    logrus.Fields{},
			}

			result, err := formatter.Format(entry)
			if err != nil {
				t.Errorf("Format() error = %v", err)
				return
			}

			// Check if color is applied correctly
			if tt.expectedColor != "" {
				if !contains(string(result), tt.expectedColor) {
					t.Errorf("Format() result = %q, expected to contain color code %q", result, tt.expectedColor)
				}
				// Check for reset code
				if !contains(string(result), "\033[0m") {
					t.Errorf("Format() result = %q, expected to contain reset code '\\033[0m'", result)
				}
			} else {
				// No color should be applied
				if contains(string(result), "\033[") {
					t.Errorf("Format() result = %q, expected no color codes", result)
				}
			}

			// Check if message is included
			if !contains(string(result), tt.message) {
				t.Errorf("Format() result = %q, expected to contain message %q", result, tt.message)
			}

			// Check if newline is included
			if !contains(string(result), "\n") {
				t.Errorf("Format() result = %q, expected to contain newline", result)
			}
		})
	}

	// Test with msg in data field
	t.Run("Message from data field", func(t *testing.T) {
		customMsg := "Message from data field"
		entry := &logrus.Entry{
			Logger:  logrus.New(),
			Level:   logrus.InfoLevel,
			Message: "Original message",
			Data:    logrus.Fields{"msg": customMsg},
		}

		result, err := formatter.Format(entry)
		if err != nil {
			t.Errorf("Format() error = %v", err)
			return
		}

		// Should use msg from data field instead of Message
		if !contains(string(result), customMsg) {
			t.Errorf("Format() result = %q, expected to contain custom msg %q", result, customMsg)
		}
		if contains(string(result), "Original message") {
			t.Errorf("Format() result = %q, should not contain original message", result)
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

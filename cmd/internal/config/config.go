package config

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/bwinhwang/githookkit"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Config contains all possible configuration options
type Config struct {
	ProjectsWhitelist []string         `yaml:"projects_whitelist"`
	ProjectSizeLimits map[string]int64 `yaml:"project_size_limits"`
	LogConfig         LogConfig        `yaml:"log_config"`
}

// LogConfig defines logging configuration
type LogConfig struct {
	Level  string `yaml:"level"`  // Log level: debug, info, warn, error
	Output string `yaml:"output"` // Log output: stdout, stderr, or file path
}

// CommandParams contains all possible command line parameters
type CommandParams struct {
	Project          string
	Uploader         string
	UploaderUsername string
	OldRev           string
	NewRev           string
	RefName          string
	CmdRef           string
}

// Logger is a wrapper around logrus.Logger that tracks open file resources
type Logger struct {
	*logrus.Logger
	file   *os.File // The file handle if logging to a file
	level  string   // Current log level
	output string   // Current output destination
}

// Close properly closes any resources held by the logger
func (l *Logger) Close() {
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() logrus.Level {
	return l.Logger.GetLevel()
}

// GetOutput returns the current output destination
func (l *Logger) GetOutput() string {
	return l.output
}

// LoadConfig loads configuration from the config file
func LoadConfig() (Config, error) {
	// Try both HOME (Linux/macOS) and USERPROFILE (Windows)
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		homeDir = os.Getenv("USERPROFILE")
	}

	configPath := filepath.Join(homeDir, ".githook_config")
	configData, err := os.ReadFile(configPath)

	config := Config{
		ProjectsWhitelist: []string{},
		ProjectSizeLimits: map[string]int64{},
	}

	if err != nil {
		log.Printf("Config file does not exist or cannot be read: %v, using empty config", err)
		return config, nil
	}

	if err := yaml.Unmarshal(configData, &config); err != nil {
		log.Printf("Failed to parse config file: %v, using empty config", err)
		return Config{
			ProjectsWhitelist: []string{},
			ProjectSizeLimits: map[string]int64{},
		}, nil
	}

	return config, nil
}

// IsProjectWhitelisted checks if a project is in the whitelist
func IsProjectWhitelisted(config Config, project string) bool {
	return Contains(config.ProjectsWhitelist, project)
}

// GetSizeLimit gets the file size limit (from env var or project-specific)
func GetSizeLimit(config Config, project string) int64 {
	// Default value 5MB
	var sizeLimit int64 = 5 * 1024 * 1024

	// From environment variable
	if envSize := os.Getenv("GITHOOK_FILE_SIZE_MAX"); envSize != "" {
		if size, err := strconv.ParseInt(envSize, 10, 64); err == nil {
			sizeLimit = size
		}
	}

	// Check project-specific size limit
	if projectLimit, exists := config.ProjectSizeLimits[project]; exists {
		fmt.Printf("Using project-specific size limit for %s: %s\n", project, githookkit.FormatSize(projectLimit))
		return projectLimit
	}

	return sizeLimit
}

// Contains checks if a string is in a slice
func Contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

// InitLogger initializes the logging system
func InitLogger(config Config) (*Logger, error) {
	// From environment variables (优先于配置文件)
	level := os.Getenv("GITHOOK_LOG_LEVEL")
	if level == "" {
		level = config.LogConfig.Level
	}
	output := os.Getenv("GITHOOK_LOG_OUTPUT")
	if output == "" {
		output = config.LogConfig.Output
	}

	// Set default values
	if level == "" {
		level = "info"
	}
	if output == "" {
		output = "stderr"
	}

	// Create logger
	logger := &Logger{
		Logger: logrus.New(),
		level:  level,
		output: output,
	}

	// Set log level
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	logger.SetLevel(logLevel)

	// Create formatters
	fileFormatter := &logrus.TextFormatter{
		FullTimestamp:          true,
		TimestampFormat:        "2006-01-02 15:04:05",
		DisableColors:          true,
		DisableLevelTruncation: true,
		PadLevelText:           true,
	}

	// Set output target
	if output == "stdout" || output == "stderr" {
		var out io.Writer
		if output == "stdout" {
			out = os.Stdout
		} else {
			out = os.Stderr
		}

		logger.SetOutput(out)
		logger.SetFormatter(&ConsoleFormatter{})
	} else {
		fileWriter, err := os.OpenFile(output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		// Store the file handle for later cleanup
		logger.file = fileWriter

		// Use MultiWriter to output to both file and stdout
		multiWriter := io.MultiWriter(fileWriter, os.Stdout)
		logger.SetOutput(multiWriter)
		logger.SetFormatter(fileFormatter)
	}

	//logger.Infof("Initialized logging system, level: %s, output: %s", level, output)

	return logger, nil
}

type ConsoleFormatter struct{}

func (f *ConsoleFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// Extract only the msg field content
	msg, exists := entry.Data["msg"]
	if !exists {
		msg = entry.Message
	}
	// Set color based on log level
	var colorCode string
	switch entry.Level {
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		colorCode = "\033[31m" // Red
	case logrus.WarnLevel:
		colorCode = "\033[33m" // Yellow
	default:
		colorCode = "" // No color
	}

	var formattedMsg string
	if colorCode != "" {
		// Add color and reset
		formattedMsg = fmt.Sprintf("%s%s\033[0m\n", colorCode, msg)
	} else {
		// No color
		formattedMsg = fmt.Sprintf("%s\n", msg)
	}

	return []byte(formattedMsg), nil
}

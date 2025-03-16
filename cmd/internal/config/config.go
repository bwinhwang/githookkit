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

// Config 包含所有可能的配置选项
type Config struct {
	ProjectsWhitelist []string         `yaml:"projects_whitelist"`
	ProjectSizeLimits map[string]int64 `yaml:"project_size_limits"`
	LogConfig         LogConfig        `yaml:"log_config"`
}

// LogConfig 定义日志配置
type LogConfig struct {
	Level  string `yaml:"level"`  // 日志级别：debug, info, warn, error
	Output string `yaml:"output"` // 日志输出：stdout, stderr, 或文件路径
}

// CommandParams 包含所有可能的命令行参数
type CommandParams struct {
	Project          string
	Uploader         string
	UploaderUsername string
	OldRev           string
	NewRev           string
	RefName          string
	CmdRef           string
}

// LoadConfig 从配置文件加载配置
func LoadConfig() (Config, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".githook_config")
	configData, err := os.ReadFile(configPath)

	config := Config{
		ProjectsWhitelist: []string{},
		ProjectSizeLimits: map[string]int64{},
	}

	if err != nil {
		log.Printf("配置文件不存在或无法读取: %v，使用空配置", err)
		return config, nil
	}

	if err := yaml.Unmarshal(configData, &config); err != nil {
		log.Printf("解析配置文件失败: %v，使用空配置", err)
		return config, nil
	}

	return config, nil
}

// IsProjectWhitelisted 检查项目是否在白名单中
func IsProjectWhitelisted(config Config, project string) bool {
	return Contains(config.ProjectsWhitelist, project)
}

// GetSizeLimit 获取文件大小限制（环境变量或项目特定）
func GetSizeLimit(config Config, project string) int64 {
	// 默认值 5MB
	var sizeLimit int64 = 5 * 1024 * 1024

	// 从环境变量获取
	if envSize := os.Getenv("GITHOOK_FILE_SIZE_MAX"); envSize != "" {
		if size, err := strconv.ParseInt(envSize, 10, 64); err == nil {
			sizeLimit = size
		}
	}

	// 检查项目特定的大小限制
	if projectLimit, exists := config.ProjectSizeLimits[project]; exists {
		fmt.Printf("Using project-specific size limit for %s: %s\n", project, githookkit.FormatSize(projectLimit))
		return projectLimit
	}

	return sizeLimit
}

// Contains 检查字符串是否在切片中
func Contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

// InitLogger 初始化日志系统
func InitLogger(config Config) (*logrus.Logger, error) {
	// 从环境变量获取日志配置（优先于配置文件）
	level := os.Getenv("GITHOOK_LOG_LEVEL")
	if level == "" {
		level = config.LogConfig.Level
	}
	output := os.Getenv("GITHOOK_LOG_OUTPUT")
	if output == "" {
		output = config.LogConfig.Output
	}

	// 设置默认值
	if level == "" {
		level = "info"
	}
	if output == "" {
		output = "stderr"
	}

	// 创建logger
	logger := logrus.New()

	// 设置日志级别
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("无效的日志级别: %w", err)
	}
	logger.SetLevel(logLevel)

	// 设置输出格式
	// 创建不同的格式化器
	fileFormatter := &logrus.TextFormatter{
		FullTimestamp:          true,
		TimestampFormat:        "2006-01-02 15:04:05",
		DisableColors:          true,
		DisableLevelTruncation: true,
		PadLevelText:           true,
	}

	// 设置输出目标
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
			return nil, fmt.Errorf("无法打开日志文件: %w", err)
		}

		// 使用MultiWriter同时输出到文件和标准输出
		multiWriter := io.MultiWriter(fileWriter, os.Stdout)
		logger.SetOutput(multiWriter)

		// 设置不同的格式化器
		logger.SetFormatter(fileFormatter)
	}

	logger.Infof("初始化日志系统，级别：%s，输出：%s", level, output)

	return logger, nil
}

type ConsoleFormatter struct{}

func (f *ConsoleFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	// 只提取msg字段的内容
	msg, exists := entry.Data["msg"]
	if !exists {
		msg = entry.Message
	}
	// 根据日志级别设置颜色
	var colorCode string
	switch entry.Level {
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		colorCode = "\033[31m" // Red
	default:
		colorCode = "\033[0m" // Reset
	}

	// 添加颜色并重置
	coloredMsg := fmt.Sprintf("%s%s\033[0m\n", colorCode, msg)
	return []byte(coloredMsg), nil
}

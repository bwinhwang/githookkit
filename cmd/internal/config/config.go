package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/bwinhwang/githookkit"
	"gopkg.in/yaml.v2"
)

// Config 包含所有可能的配置选项
type Config struct {
	ProjectsWhitelist []string         `yaml:"projects_whitelist"`
	ProjectSizeLimits map[string]int64 `yaml:"project_size_limits"`
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

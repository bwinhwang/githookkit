package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 创建临时配置文件
	homeDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", homeDir)
	defer os.Setenv("HOME", oldHome)

	configPath := filepath.Join(homeDir, ".githook_config")

	// 测试1: 配置文件不存在的情况
	config, err := LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() 返回了错误: %v", err)
	}
	if len(config.ProjectsWhitelist) != 0 {
		t.Errorf("配置文件不存在时，ProjectsWhitelist 应为空")
	}
	if len(config.ProjectSizeLimits) != 0 {
		t.Errorf("配置文件不存在时，ProjectSizeLimits 应为空")
	}

	// 测试2: 有效的配置文件
	validConfig := `
projects_whitelist:
  - project1
  - project2
project_size_limits:
  project1: 10485760
  project2: 20971520
`
	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("无法创建测试配置文件: %v", err)
	}

	config, err = LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() 返回了错误: %v", err)
	}
	if len(config.ProjectsWhitelist) != 2 {
		t.Errorf("ProjectsWhitelist 长度应为 2，实际为 %d", len(config.ProjectsWhitelist))
	}
	if config.ProjectsWhitelist[0] != "project1" || config.ProjectsWhitelist[1] != "project2" {
		t.Errorf("ProjectsWhitelist 内容不正确")
	}
	if len(config.ProjectSizeLimits) != 2 {
		t.Errorf("ProjectSizeLimits 长度应为 2，实际为 %d", len(config.ProjectSizeLimits))
	}
	if config.ProjectSizeLimits["project1"] != 10485760 || config.ProjectSizeLimits["project2"] != 20971520 {
		t.Errorf("ProjectSizeLimits 内容不正确")
	}

	// 测试3: 无效的配置文件
	invalidConfig := `invalid yaml content`
	if err := os.WriteFile(configPath, []byte(invalidConfig), 0644); err != nil {
		t.Fatalf("无法创建测试配置文件: %v", err)
	}

	config, err = LoadConfig()
	if err != nil {
		t.Errorf("LoadConfig() 返回了错误: %v", err)
	}
	if len(config.ProjectsWhitelist) != 0 {
		t.Errorf("无效配置文件时，ProjectsWhitelist 应为空")
	}
	if len(config.ProjectSizeLimits) != 0 {
		t.Errorf("无效配置文件时，ProjectSizeLimits 应为空")
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
			t.Errorf("IsProjectWhitelisted(%s) = %v, 期望 %v", test.project, result, test.expected)
		}
	}
}

func TestGetSizeLimit(t *testing.T) {
	// 保存原始环境变量
	oldEnv := os.Getenv("GITHOOK_FILE_SIZE_MAX")
	defer os.Setenv("GITHOOK_FILE_SIZE_MAX", oldEnv)

	config := Config{
		ProjectSizeLimits: map[string]int64{
			"project1": 10 * 1024 * 1024,
			"project2": 20 * 1024 * 1024,
		},
	}

	// 测试1: 使用默认值
	os.Unsetenv("GITHOOK_FILE_SIZE_MAX")
	result := GetSizeLimit(config, "project3")
	if result != 5*1024*1024 {
		t.Errorf("GetSizeLimit(project3) = %d, 期望 %d", result, 5*1024*1024)
	}

	// 测试2: 使用环境变量
	os.Setenv("GITHOOK_FILE_SIZE_MAX", "15728640") // 15MB
	result = GetSizeLimit(config, "project3")
	if result != 15*1024*1024 {
		t.Errorf("GetSizeLimit(project3) = %d, 期望 %d", result, 15*1024*1024)
	}

	// 测试3: 使用项目特定限制
	result = GetSizeLimit(config, "project1")
	if result != 10*1024*1024 {
		t.Errorf("GetSizeLimit(project1) = %d, 期望 %d", result, 10*1024*1024)
	}

	result = GetSizeLimit(config, "project2")
	if result != 20*1024*1024 {
		t.Errorf("GetSizeLimit(project2) = %d, 期望 %d", result, 20*1024*1024)
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

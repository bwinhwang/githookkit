package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainIntegration(t *testing.T) {
	// 保存当前工作目录
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("获取当前工作目录失败: %v", err)
	}
	defer os.Chdir(originalWd)

	// 切换到测试仓库目录
	repoPath := filepath.Join("..", "..", "testdata", "meta-ti")
	err = os.Chdir(repoPath)
	if err != nil {
		t.Fatalf("切换到测试仓库目录失败: %v", err)
	}

	// 编译可执行文件
	tempDir, err := os.MkdirTemp("", "githook-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 根据操作系统添加适当的扩展名
	execName := "commit-received"
	if os.PathSeparator == '\\' { // Windows系统
		execName += ".exe"
	}
	execPath := filepath.Join(tempDir, execName)

	if err := compileExecutable(originalWd, execPath); err != nil {
		t.Fatalf("编译可执行文件失败: %v", err)
	}

	// 确认可执行文件存在
	if _, err := os.Stat(execPath); os.IsNotExist(err) {
		t.Fatalf("编译后的可执行文件不存在: %s", execPath)
	}

	// 创建测试配置文件
	configPath := filepath.Join(tempDir, ".githook_config")
	if err := createTestConfig(configPath); err != nil {
		t.Fatalf("创建测试配置文件失败: %v", err)
	}

	// 测试用例
	tests := []struct {
		name           string
		args           []string
		env            []string
		expectedOutput []string
		notExpected    []string
		wantErr        bool
	}{
		{
			name: "基本参数测试",
			args: []string{
				"-project", "test-project",
				"-uploader", "Test User",
				"-uploader-username", "testuser",
				"-oldrev", "7d39ce1743e1a58c51b35f42fb70f9e31a4c8908",
				"-newrev", "HEAD",
				"-refname", "refs/heads/master",
				"-cmdref", "refs/heads/master",
			},
			env: []string{
				"GITHOOK_FILE_SIZE_MAX=32768", // 32KB
				fmt.Sprintf("HOME=%s", tempDir),
			},
			expectedOutput: []string{
				"Project name: test-project",
				"Uploader information: Test User",
				"Uploader username: testuser",
				"Command reference: refs/heads/master",
				"Found", // 应该找到一些大文件
			},
			wantErr: false,
		},
		{
			name: "白名单项目测试",
			args: []string{
				"-project", "whitelisted-project",
				"-uploader", "Test User",
				"-uploader-username", "testuser",
				"-oldrev", "7d39ce1743e1a58c51b35f42fb70f9e31a4c8908",
				"-newrev", "HEAD",
				"-refname", "refs/heads/master",
				"-cmdref", "refs/heads/master",
			},
			env: []string{
				"GITHOOK_FILE_SIZE_MAX=2048",
				fmt.Sprintf("HOME=%s", tempDir),
			},
			expectedOutput: []string{
				"Project whitelisted-project is in the whitelist, exiting",
			},
			notExpected: []string{
				"Found", // 不应该执行文件检查
			},
			wantErr: false,
		},
		{
			name: "自定义大小限制测试",
			args: []string{
				"-project", "test-project",
				"-uploader", "Test User",
				"-uploader-username", "testuser",
				"-oldrev", "7d39ce1743e1a58c51b35f42fb70f9e31a4c8908",
				"-newrev", "HEAD",
				"-refname", "refs/heads/master",
			},
			env: []string{
				"GITHOOK_FILE_SIZE_MAX=65536", // 64KB
				fmt.Sprintf("HOME=%s", tempDir),
			},
			expectedOutput: []string{
				"Project name: test-project",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(execPath, tt.args...)
			cmd.Env = append(os.Environ(), tt.env...)

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			output := stdout.String() + stderr.String()

			// 检查错误
			if (err != nil) != tt.wantErr {
				t.Errorf("命令执行错误 = %v, 期望错误 %v", err, tt.wantErr)
				t.Logf("输出: %s", output)
				return
			}

			// 检查期望的输出
			for _, expected := range tt.expectedOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("输出中未找到期望的字符串: %s", expected)
					t.Logf("实际输出: %s", output)
				}
			}

			// 检查不应该出现的输出
			for _, notExpected := range tt.notExpected {
				if strings.Contains(output, notExpected) {
					t.Errorf("输出中不应该包含字符串: %s", notExpected)
					t.Logf("实际输出: %s", output)
				}
			}

			//t.Logf("测试 '%s' 输出:\n%s", tt.name, output)
		})
	}
}

// 编译可执行文件
func compileExecutable(sourceDir, outputPath string) error {
	cmd := exec.Command("go", "build", "-o", outputPath)
	cmd.Dir = sourceDir
	return cmd.Run()
}

// 创建测试配置文件
func createTestConfig(configPath string) error {
	config := `projects_whitelist:
  - whitelisted-project
  - another-whitelisted-project
`
	return os.WriteFile(configPath, []byte(config), 0644)
}

// 测试命令行参数解析
func TestCommandLineArgs(t *testing.T) {
	// 保存原始参数和输出
	oldArgs := os.Args
	oldStdout := os.Stdout
	defer func() {
		os.Args = oldArgs
		os.Stdout = oldStdout
	}()

	// 创建管道捕获输出
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 设置测试参数
	os.Args = []string{
		"commit-received",
		"-project", "test-project",
		"-uploader", "Test User",
		"-uploader-username", "testuser",
		"-oldrev", "dummy-old-rev",
		"-newrev", "dummy-new-rev",
		"-refname", "refs/heads/master",
		"-cmdref", "refs/heads/master",
	}

	// 定义与main.go中相同的标志
	projectName := flag.String("project", "", "项目名称")
	uploader := flag.String("uploader", "", "上传者信息")
	uploaderUsername := flag.String("uploader-username", "", "上传者用户名")
	oldRev := flag.String("oldrev", "", "旧版本哈希")
	newRev := flag.String("newrev", "", "新版本哈希")
	refName := flag.String("refname", "", "引用名称")
	cmdRef := flag.String("cmdref", "", "命令引用名称")

	// 解析标志
	flag.Parse()

	// 验证标志值
	if *projectName != "test-project" {
		t.Errorf("项目名称解析错误，期望 'test-project'，得到 '%s'", *projectName)
	}
	if *uploader != "Test User" {
		t.Errorf("上传者信息解析错误，期望 'Test User'，得到 '%s'", *uploader)
	}
	if *uploaderUsername != "testuser" {
		t.Errorf("上传者用户名解析错误，期望 'testuser'，得到 '%s'", *uploaderUsername)
	}
	if *oldRev != "dummy-old-rev" {
		t.Errorf("旧版本哈希解析错误，期望 'dummy-old-rev'，得到 '%s'", *oldRev)
	}
	if *newRev != "dummy-new-rev" {
		t.Errorf("新版本哈希解析错误，期望 'dummy-new-rev'，得到 '%s'", *newRev)
	}
	if *refName != "refs/heads/master" {
		t.Errorf("引用名称解析错误，期望 'refs/heads/master'，得到 '%s'", *refName)
	}
	if *cmdRef != "refs/heads/master" {
		t.Errorf("命令引用名称解析错误，期望 'refs/heads/master'，得到 '%s'", *cmdRef)
	}

	// 输出解析结果
	fmt.Printf("项目名称: %s\n", *projectName)
	fmt.Printf("上传者信息: %s\n", *uploader)
	fmt.Printf("上传者用户名: %s\n", *uploaderUsername)
	fmt.Printf("旧版本哈希: %s\n", *oldRev)
	fmt.Printf("新版本哈希: %s\n", *newRev)
	fmt.Printf("引用名称: %s\n", *refName)
	fmt.Printf("命令引用名称: %s\n", *cmdRef)

	w.Close()

	// 读取捕获的输出
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// 验证输出中包含参数值
	expectedStrings := []string{
		"test-project",
		"Test User",
		"testuser",
		"dummy-old-rev",
		"dummy-new-rev",
		"refs/heads/master",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("输出中未找到期望的参数值: %s", expected)
		}
	}
}

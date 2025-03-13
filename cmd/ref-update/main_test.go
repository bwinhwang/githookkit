package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
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

	// 测试用例
	tests := []struct {
		name        string
		startCommit string
		endCommit   string
		sizeFilter  func(int64) bool
		minResults  int
		maxResults  int
		wantErr     bool
	}{
		{
			name:        "大于2KB的文件",
			startCommit: "HEAD~5",
			endCommit:   "HEAD",
			sizeFilter: func(size int64) bool {
				return size > 2*1024 // 大于2KB的文件
			},
			minResults: 1,
			maxResults: 10,
			wantErr:    false,
		},
		{
			name:        "小于1KB的文件",
			startCommit: "HEAD~5",
			endCommit:   "HEAD",
			sizeFilter: func(size int64) bool {
				return size < 1024 // 小于1KB的文件
			},
			minResults: 5,
			maxResults: 20,
			wantErr:    false,
		},
		{
			name:        "所有文件（无过滤）",
			startCommit: "HEAD~5",
			endCommit:   "HEAD",
			sizeFilter: func(size int64) bool {
				return true // 所有文件
			},
			minResults: 6,
			maxResults: 100,
			wantErr:    false,
		},
		{
			name:        "无文件（过滤所有）",
			startCommit: "HEAD~5",
			endCommit:   "HEAD",
			sizeFilter: func(size int64) bool {
				return false // 过滤所有文件
			},
			minResults: 0,
			maxResults: 0,
			wantErr:    false,
		},
		{
			name:        "无效的起始提交",
			startCommit: "nonexistent",
			endCommit:   "HEAD",
			sizeFilter: func(size int64) bool {
				return true
			},
			wantErr: true,
		},
		{
			name:        "所有大于2KB的文件",
			startCommit: "7d39ce1743e1a58c51b35f42fb70f9e31a4c8908",
			endCommit:   "HEAD",
			sizeFilter: func(size int64) bool {
				return size > 2*1024
			},
			minResults: 4385,
			maxResults: 4385,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := run(tt.startCommit, tt.endCommit, tt.sizeFilter)

			// 检查错误
			if (err != nil) != tt.wantErr {
				t.Errorf("run() 错误 = %v, 期望错误 %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			// 检查结果数量
			if len(results) < tt.minResults || len(results) > tt.maxResults {
				t.Errorf("run() 返回 %d 个结果, 期望范围 [%d, %d]", len(results), tt.minResults, tt.maxResults)
			}

			// 验证结果中的每个文件信息
			for _, info := range results {
				// 确保路径不为空
				if info.Path == "" {
					t.Errorf("run() 返回了空路径的文件信息")
				}

				// 确保大小符合过滤条件
				if !tt.sizeFilter(info.Size) {
					t.Errorf("run() 返回的文件 %s 大小为 %d，不符合过滤条件", info.Path, info.Size)
				}

				// 确保哈希不为空
				if info.Hash == "" {
					t.Errorf("run() 返回了空哈希的文件信息: %s", info.Path)
				}
			}

			// 打印一些调试信息
			t.Logf("找到 %d 个符合条件的文件", len(results))
			for i, info := range results {
				if i < 5 { // 只打印前5个，避免输出过多
					t.Logf("文件 #%d: 路径=%s, 大小=%d 字节, 哈希=%s", i+1, info.Path, info.Size, info.Hash)
				}
			}
		})
	}
}

func TestRunWithSpecificCommits(t *testing.T) {
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

	// 使用特定的提交范围
	startCommit := "7d39ce1743e1a58c51b35f42fb70f9e31a4c8908"
	endCommit := "HEAD"

	// 测试不同大小阈值的过滤器
	thresholds := []int64{500, 1000, 2000, 5000}

	for _, threshold := range thresholds {
		t.Run(fmt.Sprintf("Threshold: %d", threshold), func(t *testing.T) {
			sizeFilter := func(size int64) bool {
				return size > threshold
			}

			results, err := run(startCommit, endCommit, sizeFilter)
			if err != nil {
				t.Fatalf("run() 错误 = %v", err)
			}

			// 验证所有返回的文件都大于阈值
			for _, info := range results {
				if info.Size <= threshold {
					t.Errorf("文件 %s 大小为 %d，小于等于阈值 %d", info.Path, info.Size, threshold)
				}
			}

			t.Logf("阈值 %d: 找到 %d 个文件", threshold, len(results))
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
				"test-project",
				"Test User",
				"testuser",
				"refs/heads/master",
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
				"test-project",
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

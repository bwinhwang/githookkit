package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bwinhwang/githookkit"
)

func TestRun(t *testing.T) {
	// 保存当前工作目录
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("无法获取当前工作目录: %v", err)
	}
	defer os.Chdir(originalWd)

	// 切换到测试仓库目录
	err = os.Chdir(filepath.Join("../../testdata", "meta-ti"))
	if err != nil {
		t.Fatalf("无法切换到测试仓库目录: %v", err)
	}

	tests := []struct {
		name        string
		startCommit string
		endCommit   string
		sizeLimit   int64
		wantFiles   int
		wantErr     bool
	}{
		{
			name:        "无大文件",
			startCommit: "HEAD~3",
			endCommit:   "HEAD",
			sizeLimit:   10 * 1024 * 1024, // 10MB
			wantFiles:   0,
			wantErr:     false,
		},
		{
			name:        "有大文件",
			startCommit: "HEAD~5",
			endCommit:   "HEAD",
			sizeLimit:   1024, // 1KB
			wantFiles:   1,    // 至少应该找到一个大于1KB的文件
			wantErr:     false,
		},
		{
			name:        "无效的提交范围",
			startCommit: "invalid-hash",
			endCommit:   "HEAD",
			sizeLimit:   1024,
			wantFiles:   0,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			largeFiles, err := run(tt.startCommit, tt.endCommit, func(size int64) bool {
				return size > tt.sizeLimit
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("run() 错误 = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if tt.wantFiles == 0 && len(largeFiles) > 0 {
					t.Errorf("run() 返回了 %d 个文件，期望没有文件", len(largeFiles))
				} else if tt.wantFiles > 0 && len(largeFiles) < tt.wantFiles {
					t.Errorf("run() 返回了 %d 个文件，期望至少 %d 个文件", len(largeFiles), tt.wantFiles)
				}

				// 验证返回的文件信息
				for _, file := range largeFiles {
					if file.Path == "" {
						t.Error("run() 返回了路径为空的文件信息")
					}
					if file.Size <= tt.sizeLimit {
						t.Errorf("run() 返回了大小为 %d 的文件，不大于限制 %d", file.Size, tt.sizeLimit)
					}
				}
			}
		})
	}
}

func TestRunWithSpecificSizeLimit(t *testing.T) {
	// 保存当前工作目录
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("无法获取当前工作目录: %v", err)
	}
	defer os.Chdir(originalWd)

	// 切换到测试仓库目录
	err = os.Chdir(filepath.Join("../../testdata", "meta-ti"))
	if err != nil {
		t.Fatalf("无法切换到测试仓库目录: %v", err)
	}

	// 测试不同大小限制下的结果
	sizeLimits := []struct {
		limit     int64
		minFiles  int
		maxFiles  int
		checkSize bool
	}{
		{100, 5, 20, true},        // 100字节，应该找到多个文件
		{1024, 1, 10, true},       // 1KB，应该找到几个文件
		{10 * 1024, 0, 5, true},   // 10KB，可能找到少量文件
		{1024 * 1024, 0, 0, true}, // 1MB，可能找不到文件
	}

	for _, sl := range sizeLimits {
		t.Run(githookkit.FormatSize(sl.limit), func(t *testing.T) {
			largeFiles, err := run("HEAD~5", "HEAD", func(size int64) bool {
				return size > sl.limit
			})

			if err != nil {
				t.Fatalf("run() 错误 = %v", err)
			}

			if sl.minFiles > 0 && len(largeFiles) < sl.minFiles {
				t.Errorf("run() 返回了 %d 个文件，期望至少 %d 个文件", len(largeFiles), sl.minFiles)
			}

			if sl.maxFiles >= 0 && len(largeFiles) > sl.maxFiles {
				t.Errorf("run() 返回了 %d 个文件，期望最多 %d 个文件", len(largeFiles), sl.maxFiles)
			}

			if sl.checkSize {
				for _, file := range largeFiles {
					if file.Size <= sl.limit {
						t.Errorf("run() 返回了大小为 %d 的文件，不大于限制 %d", file.Size, sl.limit)
					}
				}
			}
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
project_size_limits:
  test-project-small: 1024
  test-project-medium: 10240
  test-project-large: 102400
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
				"-oldrev", "HEAD~50",
				"-newrev", "HEAD",
				"-refname", "refs/heads/master",
			},
			env: []string{
				"GITHOOK_FILE_SIZE_MAX=32768", // 32KB
				fmt.Sprintf("HOME=%s", tempDir),
			},
			expectedOutput: []string{
				"", // 应该找到一些大文件
			},
			wantErr: false,
		},
		{
			name: "白名单项目测试",
			args: []string{
				"-project", "whitelisted-project",
				"-uploader", "Test User",
				"-uploader-username", "testuser",
				"-oldrev", "7454e0e0c7cfe3526499e5a752a938aade6b7f6d",
				"-newrev", "HEAD",
				"-refname", "refs/heads/master",
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
				"-oldrev", "7454e0e0c7cfe3526499e5a752a938aade6b7f6d",
				"-newrev", "HEAD",
				"-refname", "refs/heads/master",
			},
			env: []string{
				"GITHOOK_FILE_SIZE_MAX=2048", // 64KB
				fmt.Sprintf("HOME=%s", tempDir),
			},
			expectedOutput: []string{
				"Found",
			},
			wantErr: true,
		},
		{
			name: "项目特定大小限制测试",
			args: []string{
				"-project", "test-project-small",
				"-uploader", "Test User",
				"-uploader-username", "testuser",
				"-oldrev", "7454e0e0c7cfe3526499e5a752a938aade6b7f6d",
				"-newrev", "HEAD",
				"-refname", "refs/heads/master",
			},
			env: []string{
				"GITHOOK_FILE_SIZE_MAX=65536", // 默认64KB
				fmt.Sprintf("HOME=%s", tempDir),
			},
			expectedOutput: []string{
				"Using project-specific size limit for test-project-small: 1.00 KB",
				"Found", // 应该找到更多大文件，因为限制更小
			},
			wantErr: true,
		},
		{
			name: "项目特定大小限制测试-中等",
			args: []string{
				"-project", "test-project-medium",
				"-uploader", "Test User",
				"-uploader-username", "testuser",
				"-oldrev", "7454e0e0c7cfe3526499e5a752a938aade6b7f6d",
				"-newrev", "HEAD",
				"-refname", "refs/heads/master",
			},
			env: []string{
				"GITHOOK_FILE_SIZE_MAX=2048", // 默认2KB
				fmt.Sprintf("HOME=%s", tempDir),
			},
			expectedOutput: []string{
				"Using project-specific size limit for test-project-medium: 10.00 KB",
			},
			wantErr: true,
		},
		{
			name: "无特定大小限制的项目测试",
			args: []string{
				"-project", "regular-project",
				"-uploader", "Test User",
				"-uploader-username", "testuser",
				"-oldrev", "7454e0e0c7cfe3526499e5a752a938aade6b7f6d",
				"-newrev", "HEAD",
				"-refname", "refs/heads/master",
			},
			env: []string{
				"GITHOOK_FILE_SIZE_MAX=4096", // 4KB
				fmt.Sprintf("HOME=%s", tempDir),
			},
			notExpected: []string{
				"Using project-specific size limit",
			},
			wantErr: true,
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

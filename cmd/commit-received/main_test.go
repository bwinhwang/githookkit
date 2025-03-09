package main

import (
	"fmt"
	"os"
	"path/filepath"
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

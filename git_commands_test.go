package githookkit

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetObjectListWithSpecificCommits(t *testing.T) {
	// 保存当前工作目录
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// 切换到测试仓库目录
	err = os.Chdir(filepath.Join("testdata", "meta-ti"))
	if err != nil {
		t.Fatalf("Failed to change to test repository directory: %v", err)
	}

	// 获取仓库中的一个具体提交哈希
	cmd := exec.Command("git", "rev-parse", "HEAD")
	headCommit, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get HEAD commit: %v", err)
	}
	headCommitStr := strings.TrimSpace(string(headCommit))

	// Get HEAD~1 commit hash
	cmd = exec.Command("git", "rev-parse", "HEAD~1")
	headMinus1Commit, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get HEAD~1 commit: %v", err)
	}
	headMinus1CommitStr := strings.TrimSpace(string(headMinus1Commit))

	t.Run("Specific commit range", func(t *testing.T) {
		objectChan, err := GetSpanObjectList(headMinus1CommitStr, headCommitStr, false)
		if err != nil {
			t.Fatalf("GetSpanObjectList() error = %v", err)
		}

		// 收集所有对象哈希
		objects := make(map[string]struct{})
		for hash := range objectChan {
			objects[hash] = struct{}{}
		}

		// 验证获取的对象列表不为空
		if len(objects) == 0 {
			t.Error("GetSpanObjectList() returned no objects")
		}

		// 验证所有返回的哈希都是有效的 git 对象
		for object := range objects {
			cmd := exec.Command("git", "cat-file", "-t", object)
			if err := cmd.Run(); err != nil {
				t.Errorf("Invalid git object hash returned: %s", object)
			}
		}
	})
}

func TestProcessObjectBatch(t *testing.T) {
	// 保存当前工作目录
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// 切换到测试仓库目录
	err = os.Chdir(filepath.Join("testdata", "meta-ti"))
	if err != nil {
		t.Fatalf("Failed to change to test repository directory: %v", err)
	}

	// 获取一些有效的文件对象哈希用于测试
	//cmd := exec.Command("git", "ls-tree", "-r", "HEAD")
	//cmd := exec.Command("git", "ls-tree", "HEAD")
	//cmd := exec.Command("git", "rev-list", "--objects", "--all", "HEAD~20..HEAD")
	cmd := exec.Command("git", "rev-list", "--objects", "--all", "7d39ce1743e1a58c51b35f42fb70f9e31a4c8908..HEAD")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get test objects: %v", err)
	}

	var objects []string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {

		/*
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 4 {
				// 获取对象哈希和路径
				hash := fields[2]
				path := fields[3]
				objects = append(objects, hash+" "+path) // 将哈希和路径连接并添加到对象列表
			}
		*/

		objects = append(objects, scanner.Text())
	}

	if len(objects) == 0 {
		t.Fatal("Failed to get any test objects")
	}
	//t.Logf("Found %d test objects", len(objects))

	t.Run("Process valid objects", func(t *testing.T) {
		resultChan := make(chan FileInfo)
		go func() {
			processObjectBatch(objects, resultChan, nil)
			close(resultChan)
		}()

		var results []FileInfo
		for info := range resultChan {
			results = append(results, info)
			//t.Logf("Received file info: Path=%s, Size=%d", info.Path, info.Size)
		}

		// 可能不是所有对象都有路径信息，所以我们不检查具体数量
		// 但至少应该有一些结果
		if len(results) == 0 {
			t.Error("processObjectBatch() returned no results for valid objects")
		}

		// 验证结果中的路径不为空
		for _, info := range results {
			if info.Path == "" {
				t.Error("processObjectBatch() returned FileInfo with empty path")
			}
			if info.Size <= 0 {
				t.Error("processObjectBatch() returned FileInfo with invalid size")
			}
		}
	})

	t.Run("Process invalid objects", func(t *testing.T) {
		invalidObjects := []string{"invalid1", "invalid2"}
		resultChan := make(chan FileInfo)
		go func() {
			processObjectBatch(invalidObjects, resultChan, nil)
			close(resultChan)
		}()

		var results []FileInfo
		for info := range resultChan {
			results = append(results, info)
		}

		// 对于无效对象，应该没有结果
		if len(results) > 0 {
			t.Errorf("processObjectBatch() returned %d results for invalid objects", len(results))
		}
	})

}

func TestGetObjectDetails(t *testing.T) {
	// 保存当前工作目录
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// 切换到测试仓库目录
	err = os.Chdir(filepath.Join("testdata", "meta-ti"))
	if err != nil {
		t.Fatalf("Failed to change to test repository directory: %v", err)
	}

	t.Run("GetObjectDetails with valid input", func(t *testing.T) {
		// 创建一个对象通道
		objectChan := make(chan string)

		// 获取一些有效的文件对象哈希
		cmd := exec.Command("git", "ls-tree", "-r", "HEAD")
		output, err := cmd.Output()
		if err != nil {
			t.Fatalf("Failed to get test objects: %v", err)
		}

		var objects []string
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 4 {
				// 获取对象哈希和路径
				hash := fields[2]
				path := fields[3]
				objects = append(objects, hash+" "+path) // 将哈希和路径连接并添加到对象列表
			}
		}

		if len(objects) == 0 {
			t.Fatal("Failed to get any test objects")
		}
		//t.Logf("Found %d test objects", len(objects))

		// 启动一个 goroutine 来发送对象哈希
		go func() {
			for _, hash := range objects {
				objectChan <- hash
			}
			close(objectChan)
		}()

		// 调用 GetObjectDetails
		fileInfoChan, err := GetObjectDetails(objectChan, func(size int64) bool {
			return true // 默认情况下，所有对象都包含
		})
		if err != nil {
			t.Fatalf("GetObjectDetails() error = %v", err)
		}

		// 收集结果
		var fileInfos []FileInfo
		for info := range fileInfoChan {
			fileInfos = append(fileInfos, info)
			//t.Logf("Received file info: Path=%s, Size=%d", info.Path, info.Size)
		}

		// 验证结果
		if len(fileInfos) == 0 {
			t.Error("GetObjectDetails() returned no results")
		}

		// 验证结果中的路径不为空
		for _, info := range fileInfos {
			if info.Path == "" {
				t.Error("GetObjectDetails() returned FileInfo with empty path")
			}
			if info.Size < 0 {
				t.Errorf("GetObjectDetails() returned FileInfo with invalid size at path %s", info.Path)
			}
		}
	})

	t.Run("GetObjectDetails with empty input", func(t *testing.T) {
		// 创建一个空的对象通道
		objectChan := make(chan string)
		close(objectChan)

		// 调用 GetObjectDetails
		fileInfoChan, err := GetObjectDetails(objectChan, func(size int64) bool {
			return true // 默认情况下，所有对象都包含
		})
		if err != nil {
			t.Fatalf("GetObjectDetails() error = %v", err)
		}

		// 收集结果
		var fileInfos []FileInfo
		for info := range fileInfoChan {
			fileInfos = append(fileInfos, info)
		}

		// 验证结果为空
		if len(fileInfos) > 0 {
			t.Errorf("GetObjectDetails() returned %d results for empty input", len(fileInfos))
		}
	})

	t.Run("GetObjectDetails with invalid input", func(t *testing.T) {
		// 创建一个包含无效对象的通道
		objectChan := make(chan string)

		// 启动一个 goroutine 来发送无效对象哈希
		go func() {
			objectChan <- "invalid1"
			objectChan <- "invalid2"
			close(objectChan)
		}()

		// 调用 GetObjectDetails
		fileInfoChan, err := GetObjectDetails(objectChan, func(size int64) bool {
			return true // 默认情况下，所有对象都包含
		})
		if err != nil {
			t.Fatalf("GetObjectDetails() error = %v", err)
		}

		// 收集结果
		var fileInfos []FileInfo
		for info := range fileInfoChan {
			fileInfos = append(fileInfos, info)
		}

		// 验证结果为空
		if len(fileInfos) > 0 {
			t.Errorf("GetObjectDetails() returned %d results for invalid input", len(fileInfos))
		}
	})
}

func TestGetObjectDetailsWithSizeFilter(t *testing.T) {
	// 保存当前工作目录
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// 切换到测试仓库目录
	err = os.Chdir(filepath.Join("testdata", "meta-ti"))
	if err != nil {
		t.Fatalf("Failed to change to test repository directory: %v", err)
	}

	// 获取所有大于1MB的文件
	objectChan, _ := GetSingleCommitObjectList("HEAD", true)

	fileInfoChan, _ := GetObjectDetails(objectChan, func(size int64) bool {
		return size > 2*1024 // 只包含大于2KB的文件
	})

	// 收集结果
	var fileInfos []FileInfo
	for fileInfo := range fileInfoChan {
		//t.Logf("path=%s size=%d", fileInfo.Path, fileInfo.Size)
		fileInfos = append(fileInfos, fileInfo)
	}

	// 验证结果为空
	if len(fileInfos) != 4501 {
		t.Errorf("fileInfos returned 4501 results, but %d found", len(fileInfos))
	}

	// 获取所有小于100KB的文件
	objectChan, _ = GetSingleCommitObjectList("HEAD", true)
	fileInfoChan, _ = GetObjectDetails(objectChan, func(size int64) bool {
		return size < 1024 // 只包含小于100KB的文件
	})
	// 收集结果
	fileInfos = fileInfos[:0]
	for fileInfo := range fileInfoChan {
		//t.Logf("path=%s size=%d", fileInfo.Path, fileInfo.Size)
		fileInfos = append(fileInfos, fileInfo)
	}

	// 验证结果为空
	if len(fileInfos) != 4222 {
		t.Errorf("fileInfos returned 4222 results, but %d found", len(fileInfos))
	}
}

func TestCountCommits(t *testing.T) {
	// 切换到测试仓库目录
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("无法获取当前工作目录: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir("testdata/meta-ti")
	if err != nil {
		t.Fatalf("无法切换到测试仓库目录: %v", err)
	}

	tests := []struct {
		name    string
		oldRev  string
		newRev  string
		want    int
		wantErr bool
	}{
		{
			name:    "有效的提交范围",
			oldRev:  "HEAD~3",
			newRev:  "HEAD",
			want:    3,
			wantErr: false,
		},
		{
			name:    "相同的提交",
			oldRev:  "HEAD",
			newRev:  "HEAD",
			want:    0,
			wantErr: false,
		},
		{
			name:    "无效的提交哈希",
			oldRev:  "invalid-hash",
			newRev:  "HEAD",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CountCommits(tt.newRev, tt.oldRev)
			if (err != nil) != tt.wantErr {
				t.Errorf("CountCommits() 错误 = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("CountCommits() = %v, 期望 %v", got, tt.want)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		size     int64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1023, "1023 B"},
		{1024, "1.00 KB"},
		{2048, "2.00 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
	}

	for _, test := range tests {
		result := FormatSize(test.size)
		if result != test.expected {
			t.Errorf("FormatSize(%d) = %s; want %s", test.size, result, test.expected)
		}
	}
}

func TestGetSingleCommitObjectList(t *testing.T) {
	// Save current working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Change to test repository directory
	err = os.Chdir(filepath.Join("testdata", "meta-ti"))
	if err != nil {
		t.Fatalf("Failed to change to test repository directory: %v", err)
	}

	// Get a valid commit hash
	cmd := exec.Command("git", "rev-parse", "HEAD")
	headCommit, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get HEAD commit: %v", err)
	}
	headCommitStr := strings.TrimSpace(string(headCommit))

	tests := []struct {
		name        string
		commit      string
		includePath bool
		wantErr     bool
		minObjects  int
	}{
		{
			name:        "Valid commit with hash only",
			commit:      headCommitStr,
			includePath: false,
			wantErr:     false,
			minObjects:  10, // Expect at least some objects
		},
		{
			name:        "Valid commit with path",
			commit:      headCommitStr,
			includePath: true,
			wantErr:     false,
			minObjects:  10,
		},
		{
			name:        "Invalid commit",
			commit:      "invalid-commit-hash",
			includePath: false,
			wantErr:     true,
			minObjects:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objectChan, err := GetSingleCommitObjectList(tt.commit, tt.includePath)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetSingleCommitObjectList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			objectCount := 0
			for obj := range objectChan {
				objectCount++
				// If includePath is true, check if there's a space (indicating path is included)
				if tt.includePath && !strings.Contains(obj, " ") && len(obj) > 40 {
					t.Errorf("Expected path in object but got: %s", obj)
				}
			}

			if objectCount < tt.minObjects {
				t.Errorf("GetSingleCommitObjectList() got %d objects, want at least %d", objectCount, tt.minObjects)
			}
		})
	}
}

func TestGetSpanObjectList(t *testing.T) {
	// Save current working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Change to test repository directory
	err = os.Chdir(filepath.Join("testdata", "meta-ti"))
	if err != nil {
		t.Fatalf("Failed to change to test repository directory: %v", err)
	}

	// Get HEAD and HEAD~1 commit hashes
	cmd := exec.Command("git", "rev-parse", "HEAD")
	headCommit, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get HEAD commit: %v", err)
	}
	headCommitStr := strings.TrimSpace(string(headCommit))

	cmd = exec.Command("git", "rev-parse", "HEAD~1")
	headMinus1Commit, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get HEAD~1 commit: %v", err)
	}
	headMinus1CommitStr := strings.TrimSpace(string(headMinus1Commit))

	tests := []struct {
		name        string
		startCommit string
		endCommit   string
		includePath bool
		wantErr     bool
		minObjects  int
	}{
		{
			name:        "Valid commit range hash only",
			startCommit: headMinus1CommitStr,
			endCommit:   headCommitStr,
			includePath: false,
			wantErr:     false,
			minObjects:  1, // At least one object difference between HEAD~1 and HEAD
		},
		{
			name:        "Valid commit range with path",
			startCommit: headMinus1CommitStr,
			endCommit:   headCommitStr,
			includePath: true,
			wantErr:     false,
			minObjects:  1,
		},
		{
			name:        "Invalid start commit",
			startCommit: "invalid-commit-hash",
			endCommit:   headCommitStr,
			includePath: false,
			wantErr:     true,
			minObjects:  0,
		},
		{
			name:        "Invalid end commit",
			startCommit: headMinus1CommitStr,
			endCommit:   "invalid-commit-hash",
			includePath: false,
			wantErr:     true,
			minObjects:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objectChan, err := GetSpanObjectList(tt.startCommit, tt.endCommit, tt.includePath)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetSpanObjectList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			objectCount := 0
			for obj := range objectChan {
				objectCount++
				// If includePath is true, check if objects with paths have spaces
				if tt.includePath && strings.Contains(obj, " ") {
					parts := strings.SplitN(obj, " ", 2)
					if len(parts[0]) != 40 { // Git hash is 40 characters
						t.Errorf("Invalid hash format in object: %s", obj)
					}
				}
			}

			if objectCount < tt.minObjects {
				t.Errorf("GetSpanObjectList() got %d objects, want at least %d", objectCount, tt.minObjects)
			}
		})
	}
}

func TestVerifyCommit(t *testing.T) {
	// Save current working directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)

	// Change to test repository directory
	err = os.Chdir(filepath.Join("testdata", "meta-ti"))
	if err != nil {
		t.Fatalf("Failed to change to test repository directory: %v", err)
	}

	// Get a valid commit hash
	cmd := exec.Command("git", "rev-parse", "HEAD")
	headCommit, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to get HEAD commit: %v", err)
	}
	headCommitStr := strings.TrimSpace(string(headCommit))

	tests := []struct {
		name   string
		commit string
		want   bool
	}{
		{
			name:   "Valid commit",
			commit: headCommitStr,
			want:   true,
		},
		{
			name:   "Invalid commit hash",
			commit: "invalid-commit-hash",
			want:   false,
		},
		{
			name:   "Empty commit hash",
			commit: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := VerifyCommit(tt.commit)
			if got != tt.want {
				t.Errorf("VerifyCommit() = %v, want %v", got, tt.want)
			}
		})
	}
}

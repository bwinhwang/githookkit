package githookkit

import (
	"bufio"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetObjectList(t *testing.T) {
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

	tests := []struct {
		name        string
		startCommit string
		endCommit   string
		wantErr     bool
		minObjects  int  // 至少应该有这么多对象
		includePath bool // 新增参数
	}{
		{
			name:        "Valid commit range",
			startCommit: "HEAD~5",
			endCommit:   "HEAD",
			wantErr:     false,
			minObjects:  40,
			includePath: false, // 设置为 false
		},
		{
			name:        "Invalid start commit",
			startCommit: "nonexistent",
			endCommit:   "HEAD",
			wantErr:     true,
		},
		{
			name:        "Empty commit range",
			startCommit: "HEAD",
			endCommit:   "HEAD",
			wantErr:     false,
			minObjects:  0,
			includePath: false, // 设置为 false
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objectChan, err := GetObjectList(tt.startCommit, tt.endCommit, tt.includePath)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetObjectList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				return
			}

			objectCount := 0
			for range objectChan {
				objectCount++
			}

			if objectCount < tt.minObjects {
				t.Errorf("GetObjectList() got %d objects, want at least %d", objectCount, tt.minObjects)
			}
		})
	}
}

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

	t.Run("Specific commit range", func(t *testing.T) {
		objectChan, err := GetObjectList(headCommitStr+"~1", headCommitStr, false) // 设置为 false
		if err != nil {
			t.Fatalf("GetObjectList() error = %v", err)
		}

		// 收集所有对象哈希
		objects := make(map[string]struct{})
		for hash := range objectChan {
			objects[hash] = struct{}{}
		}

		// 验证获取的对象列表不为空
		if len(objects) == 0 {
			t.Error("GetObjectList() returned no objects")
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
	objectChan, _ := GetObjectList("HEAD~5", "HEAD", true)

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
	if len(fileInfos) != 1 {
		t.Errorf("fileInfos returned ONE results, but %d found", len(fileInfos))
	}

	// 获取所有小于100KB的文件
	objectChan, _ = GetObjectList("HEAD~5", "HEAD", true)
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
	if len(fileInfos) != 5 {
		t.Errorf("fileInfos returned FIVE results, but %d found", len(fileInfos))
	}
}

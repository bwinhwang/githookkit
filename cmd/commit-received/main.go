package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/bwinhwang/githookkit"
)

func main() {
	// Define command line parameters
	project := flag.String("project", "", "Project name")
	uploader := flag.String("uploader", "", "Uploader information")
	uploaderUsername := flag.String("uploader-username", "", "Uploader username")
	oldRev := flag.String("oldrev", "", "Old commit hash")
	newRev := flag.String("newrev", "", "New commit hash")
	refName := flag.String("refname", "", "Reference name")

	// Parse command line parameters
	flag.Parse()

	// Print parameters for debugging
	fmt.Println("Project name:", *project)
	fmt.Println("Uploader information:", *uploader)
	fmt.Println("Uploader username:", *uploaderUsername)
	fmt.Println("Old commit hash:", *oldRev)
	fmt.Println("New commit hash:", *newRev)
	fmt.Println("Reference name:", *refName)

	largeFiles, err := run(*oldRev, *newRev, func(size int64) bool {
		return size > 5*1024 // 只包含大于1MB的文件
	})

	if err != nil {
		log.Fatalf("运行失败: %v", err)
	}

	// 打印结果
	fmt.Printf("找到 %d 个大文件:\n", len(largeFiles))
	for _, file := range largeFiles {
		fmt.Printf("路径: %s, 大小: %d 字节, 哈希: %s\n", file.Path, file.Size, file.Hash)
	}
}

func run(startCommit, endCommit string, sizeChecker func(int64) bool) ([]githookkit.FileInfo, error) {
	// 获取所有对象
	objectChan, err := githookkit.GetObjectList(startCommit, endCommit, true)
	if err != nil {
		return nil, fmt.Errorf("获取对象列表失败: %w", err)
	}

	// 使用 GetObjectDetails 和大小检查器过滤对象
	fileInfoChan, err := githookkit.GetObjectDetails(objectChan, sizeChecker)
	if err != nil {
		return nil, fmt.Errorf("获取对象详情失败: %w", err)
	}

	// 收集所有符合条件的文件信息
	var results []githookkit.FileInfo
	for fileInfo := range fileInfoChan {
		// 确保对象有路径和大小信息
		if fileInfo.Path != "" {
			results = append(results, fileInfo)
		}
	}

	return results, nil
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/bwinhwang/githookkit"
	"github.com/bwinhwang/githookkit/cmd/internal/config"
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

	cfg, _ := config.LoadConfig()

	// 初始化日志
	logger, err := config.InitLogger(cfg)
	if err != nil {
		fmt.Printf("初始化日志失败: %v", err)
		os.Exit(1)
	}

	// Print parameters for logging
	logger.Debugf("project=%s, ref=%s\n", *project, *refName)
	logger.Debugf("uploader=%s, username=%s\n", *uploader, *uploaderUsername)
	logger.Debugf("oldRev=%s\n", *oldRev)
	logger.Debugf("newRev=%s\n", *newRev)

	if config.IsProjectWhitelisted(cfg, *project) {
		logger.Infof("Project %s is in the whitelist, exiting\n", *project)
		os.Exit(0) // Exit normally, no error
	}

	sizeLimit := config.GetSizeLimit(cfg, *project)

	largeFiles, err := run(*oldRev, *newRev, func(size int64) bool {
		return size > sizeLimit // Use environment variable or default value
	})

	if err != nil {
		logger.Fatalf("Run failed: %v", err)
	}

	var maxFileSize int64 = 0
	if len(largeFiles) > 0 {
		logger.Infof("Found %d large files:", len(largeFiles))
		for _, file := range largeFiles {
			if file.Size > maxFileSize {
				maxFileSize = file.Size
			}

			logger.Infof("  Path: %s, Size: %d bytes", file.Path, file.Size)

		}
		logger.Fatalf("REJECTED: one or more files exceed maximum size of %s, the largest one is %s, use git lfs!", githookkit.FormatSize(sizeLimit), githookkit.FormatSize(maxFileSize))
	}
}

func run(startCommit, endCommit string, sizeChecker func(int64) bool) ([]githookkit.FileInfo, error) {
	// Get all objects
	// Collect all matching file information
	var results []githookkit.FileInfo

	// branch deletion, return
	if endCommit == "0000000000000000000000000000000000000000" {
		return results, nil
	}

	count, err := githookkit.CountCommits(endCommit, startCommit)
	if err != nil {
		return nil, fmt.Errorf("failed to get count: %w", err)
	}
	assuredStartCommit := fmt.Sprintf("%s~%d", endCommit, count)

	var objectChan <-chan string
	isOk := githookkit.VerifyCommit(assuredStartCommit)

	if isOk {
		objectChan, err = githookkit.GetSpanObjectList(assuredStartCommit, endCommit, true)

	} else {
		objectChan, err = githookkit.GetSingleCommitObjectList(endCommit, true)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get object list: %w", err)
	}

	// Use GetObjectDetails and size checker to filter objects
	fileInfoChan, err := githookkit.GetObjectDetails(objectChan, sizeChecker)
	if err != nil {
		return nil, fmt.Errorf("failed to get object details: %w", err)
	}

	for fileInfo := range fileInfoChan {
		// Ensure object has path and size information
		if fileInfo.Path != "" {
			results = append(results, fileInfo)
		}
	}

	return results, nil
}

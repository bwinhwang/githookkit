package main

import (
	"flag"
	"fmt"
	"log"
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

	// Print parameters for logging
	fmt.Printf("project=%s, ref=%s\n", *project, *refName)
	fmt.Printf("uploader=%s, username=%s\n", *uploader, *uploaderUsername)
	fmt.Printf("oldRev=%s\n", *oldRev)
	fmt.Printf("newRev=%s\n", *newRev)

	cfg, _ := config.LoadConfig()

	if config.IsProjectWhitelisted(cfg, *project) {
		fmt.Printf("Project %s is in the whitelist, exiting\n", *project)
		os.Exit(0) // Exit normally, no error
	}

	sizeLimit := config.GetSizeLimit(cfg, *project)

	largeFiles, err := run(*oldRev, *newRev, func(size int64) bool {
		return size > sizeLimit // Use environment variable or default value
	})

	if err != nil {
		log.Fatalf("Run failed: %v", err)
	}

	var maxFileSize int64 = 0
	if len(largeFiles) > 0 {
		fmt.Printf("Found %d large files:\n", len(largeFiles))
		for _, file := range largeFiles {
			if file.Size > maxFileSize {
				maxFileSize = file.Size
			}
			fmt.Printf("\tPath: %s, Size: %d bytes, Hash: %s\n", file.Path, file.Size, file.Hash)
		}
		fmt.Printf("REJECTED: one or more files exceed maximum size of %s, the largest one is %s\n", githookkit.FormatSize(sizeLimit), githookkit.FormatSize(maxFileSize))
		os.Exit(1)
	}
}

func run(startCommit, endCommit string, sizeChecker func(int64) bool) ([]githookkit.FileInfo, error) {
	// Get all objects
	objectChan, err := githookkit.GetObjectList(startCommit, endCommit, true)
	if err != nil {
		return nil, fmt.Errorf("Failed to get object list: %w", err)
	}

	// Use GetObjectDetails and size checker to filter objects
	fileInfoChan, err := githookkit.GetObjectDetails(objectChan, sizeChecker)
	if err != nil {
		return nil, fmt.Errorf("Failed to get object details: %w", err)
	}

	// Collect all matching file information
	var results []githookkit.FileInfo
	for fileInfo := range fileInfoChan {
		// Ensure object has path and size information
		if fileInfo.Path != "" {
			results = append(results, fileInfo)
		}
	}

	return results, nil
}

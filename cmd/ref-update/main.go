package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/bwinhwang/githookkit"
	"gopkg.in/yaml.v2"
)

// Define configuration struct
type Config struct {
	ProjectsWhitelist []string `yaml:"projects_whitelist"`
}

func main() {
	// Define command line parameters
	project := flag.String("project", "", "Project name")
	uploader := flag.String("uploader", "", "Uploader information")
	uploaderUsername := flag.String("uploader-username", "", "Uploader username")
	oldRev := flag.String("oldrev", "", "Old commit hash")
	newRev := flag.String("newrev", "", "New commit hash")
	refName := flag.String("refname", "", "Reference name")
	cmdRef := flag.String("cmdref", "", "Command reference name")

	// Parse command line parameters
	flag.Parse()

	// Print parameters for logging
	log.Printf("project=%s, uploader=%s, username=%s, ref=%s, cmdref=%s",
		*project, *uploader, *uploaderUsername, *refName, *cmdRef)

	// Get file size limit from environment variable, default to 5MB if not set
	var sizeLimit int64 = 5 * 1024 * 1024 // Default value 5MB
	if envSize := os.Getenv("GITHOOK_FILE_SIZE_MAX"); envSize != "" {
		if size, err := strconv.ParseInt(envSize, 10, 64); err == nil {
			sizeLimit = size
		}
	}

	configPath := filepath.Join(os.Getenv("HOME"), ".githook_config")
	configData, err := os.ReadFile(configPath)

	var config Config
	if err != nil {
		// Do not report error if config file does not exist, use empty config
		log.Printf("Config file does not exist or cannot be read: %v, using empty config", err)
		config = Config{ProjectsWhitelist: []string{}}
	} else {
		if err := yaml.Unmarshal(configData, &config); err != nil {
			// Do not report error if parsing fails, use empty config
			log.Printf("Failed to parse config file: %v, using empty config", err)
			config = Config{ProjectsWhitelist: []string{}}
		}
	}

	// Check if project name is in the whitelist
	if contains(config.ProjectsWhitelist, *project) {
		fmt.Printf("Project %s is in the whitelist, exiting\n", *project)
		os.Exit(0) // Exit normally, no error
	}

	largeFiles, err := run(*oldRev, *newRev, func(size int64) bool {
		return size > sizeLimit // Use environment variable or default value
	})

	if err != nil {
		log.Fatalf("Run failed: %v", err)
	}

	// Print results
	if len(largeFiles) > 0 {
		fmt.Printf("Found %d large files:\n", len(largeFiles))
		for _, file := range largeFiles {
			fmt.Printf("Path: %s, Size: %d bytes, Hash: %s\n", file.Path, file.Size, file.Hash)
		}
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

// Add a helper function to check if project is in the whitelist
func contains(slice []string, item string) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

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

	count, err := githookkit.CountCommits(*newRev, *oldRev)

	if err != nil {
		log.Fatalf("Run failed: %v", err)
	}

	fmt.Printf("commits: %d", count)

}

package githookkit

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// File information structure
type FileInfo struct {
	Size int64
	Path string
}

// Format file size to human-readable format
func FormatSize(size int64) string {
	const (
		B  = 1
		KB = 1024 * B
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
}
func CountCommits(newRev, oldRev string) (int, error) {

	var cmds []string
	cmds = append(cmds, "git")
	cmds = append(cmds, "rev-list")
	cmds = append(cmds, "--count")

	if oldRev == "0000000000000000000000000000000000000000" {
		cmds = append(cmds, newRev)
		cmds = append(cmds, "--not")
		cmds = append(cmds, "--all")
	} else {
		cmds = append(cmds, fmt.Sprintf("%s..%s", oldRev, newRev))
	}
	cmd := exec.Command(cmds[0], cmds[1:]...)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to execute git rev-list: %w", err)
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}

	return count, nil
}

// GetObjectList returns a channel of object hashes in the specified commit range
func GetObjectList(counts int, endCommit string, includePath bool) (<-chan string, error) {

	startCommit := fmt.Sprintf("%s~%d", endCommit, counts)
	validateCmd := exec.Command("git", "rev-parse", "--verify", startCommit)
	if err := validateCmd.Run(); err != nil {
		return nil, fmt.Errorf("invalid start commit %s: %w", startCommit, err)
	}

	validateCmd = exec.Command("git", "rev-parse", "--verify", endCommit)
	if err := validateCmd.Run(); err != nil {
		return nil, fmt.Errorf("invalid end commit %s: %w", endCommit, err)
	}

	var cmds []string
	cmds = append(cmds, "git")
	cmds = append(cmds, "rev-list")
	cmds = append(cmds, "--objects")
	cmds = append(cmds, fmt.Sprintf("%s..%s", startCommit, endCommit))

	fmt.Printf("%s\n", strings.Join(cmds, " "))
	cmd := exec.Command(cmds[0], cmds[1:]...)
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	objectChan := make(chan string)

	if err := cmd.Start(); err != nil {
		output.Close()
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	go func() {
		defer close(objectChan)
		defer output.Close()

		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			line := scanner.Text()
			if includePath {
				objectChan <- line // 发送包含路径的行
			} else {
				parts := strings.Fields(line)
				if len(parts) > 0 {
					objectChan <- parts[0] // 仅发送哈希
				}
			}
		}

		if err := cmd.Wait(); err != nil {
			return
		}
	}()

	return objectChan, nil
}

// GetObjectDetails processes objects in batches and returns a channel of FileInfo
// sizeFilter is an optional function that returns true if the object should be included based on its size
func GetObjectDetails(objectChan <-chan string, sizeFilter func(int64) bool) (<-chan FileInfo, error) {
	const batchSize = 1000
	resultChan := make(chan FileInfo)

	go func() {
		defer close(resultChan)

		var batch []string
		for line := range objectChan {
			batch = append(batch, line)

			if len(batch) >= batchSize {
				processObjectBatch(batch, resultChan, sizeFilter)
				batch = nil
			}
		}

		// Process remaining objects
		if len(batch) > 0 {
			processObjectBatch(batch, resultChan, sizeFilter)
		}
	}()

	return resultChan, nil
}

// Helper function to process a batch of objects
// sizeFilter is an optional function that returns true if the object should be included based on its size
func processObjectBatch(objects []string, resultChan chan<- FileInfo, sizeFilter func(int64) bool) {
	if len(objects) == 0 {
		return
	}

	input := strings.Join(objects, "\n")
	cmd := exec.Command("git", "cat-file", "--batch-check=%(objectname) %(objectsize) %(objecttype) %(rest)")
	cmd.Stdin = strings.NewReader(input)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	re := regexp.MustCompile(`^([a-f0-9]+) (\d+) (blob|tree)(?: (.+))?$`)

	for scanner.Scan() {
		line := scanner.Text()
		//fmt.Printf("Debug: Processing line: %s\n", line)

		matches := re.FindStringSubmatch(line)
		if len(matches) >= 4 {
			//hash := matches[1]
			size, _ := strconv.ParseInt(matches[2], 10, 64)
			objType := matches[3]
			var path string
			if len(matches) == 5 {
				path = matches[4]
			}

			//fmt.Printf("Debug: Parsed: size=%d, type=%s, path=%s\n", size, objType, path)

			// 应用大小过滤条件（如果提供）
			if objType == "blob" && path != "" && (sizeFilter == nil || sizeFilter(size)) {
				resultChan <- FileInfo{
					Size: size,
					Path: path,
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Debug: Error scanning output: %v\n", err)
	}
}

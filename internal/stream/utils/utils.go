package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func EnsureOutputDirectoryExists(mediaID string, suffix string) (string, error) {
	tempDir := os.TempDir()

	// Add /private prefix since `os.TempDir` return a symlink on macOS & ffmpeg doesn't like it
	if runtime.GOOS == "darwin" {
		tempDir = filepath.Join("/private", tempDir)
	}
	outputDir := filepath.Join(tempDir, mediaID)

	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		return "", errors.New("Unable to generate segments output directory: " + err.Error())
	}

	return outputDir, nil
}

func WaitForFile(filePath string) error {
	maxWaitDuration := 30 * time.Second // Maximum duration to wait for the file
	pollingInterval := 1 * time.Second  // Interval for checking the file existence

	startTime := time.Now()

	for {
		// File exists, stop waiting
		if FileExists(filePath) {
			return nil
		}

		if time.Since(startTime) > maxWaitDuration {
			// Timeout reached, file not found
			return fmt.Errorf("file %s not found after waiting", filePath)
		}

		time.Sleep(pollingInterval)
	}
}

func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)

	if os.IsNotExist(err) {
		return false
	}

	return err == nil
}

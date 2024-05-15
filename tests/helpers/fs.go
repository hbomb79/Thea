package helpers

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TempDirWithFiles(t *testing.T, files []string) (string, []string) {
	dirPath := t.TempDir()
	filePaths := make([]string, 0, len(files))
	for _, filename := range files {
		fileName, err := os.CreateTemp(dirPath, "*"+filename)
		assert.Nil(t, err, "failed to create temporary file in temporary dir")
		filePaths = append(filePaths, fileName.Name())
	}

	assert.Len(t, filePaths, len(files), "Expected file paths recorded to match length of requested files")
	return dirPath, filePaths
}

package helpers

import (
	"io"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TempDirWithEmptyFiles(t *testing.T, files []string) (string, []string) {
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

// TempDirWithFiles will attempt to create a temp directory
// using the testing.T instance, and populate it with *existing files
// from the host FS*.
//
// The input 'files' map is a mapping from the *host filepath* TO the
// name of the file inside of the temporary testing dir.
//
// On success, the function returns the path to the temp dir, and the list
// of file paths representing the files copied to the testing temp dir.
//
// On failure, the testing instance provided will be failed.
func TempDirWithFiles(t *testing.T, files map[string]string) (string, []string) {
	dirPath := t.TempDir()
	t.Logf("seeding temp data for %s (%s) in to dir %s...", t.Name(), files, dirPath)

	filePaths := make([]string, 0, len(files))
	for origin, desiredName := range files {
		// Open origin file
		originFile, err := os.Open(origin)
		if err != nil {
			t.Fatalf("failed to open origin path '%s': %v", origin, err)
		}

		// Create dest file with desired name
		destFile, err := os.Create(path.Join(dirPath, desiredName))
		if err != nil {
			t.Fatalf("failed to create destination file '%s' in temp dir '%s': %v", desiredName, dirPath, err)
		}

		// Copy data
		t.Logf("seeding temp file '%s' (from source %s)...", destFile.Name(), originFile.Name())
		if _, err := io.Copy(destFile, originFile); err != nil {
			t.Fatalf("failed to copy data from origin file '%s' to destination file '%s': %v", originFile.Name(), destFile.Name(), err)
		}
		if err := destFile.Sync(); err != nil {
			t.Fatalf("failed to sync fs data for file '%s': %v", destFile.Name(), err)
		}
		filePaths = append(filePaths, destFile.Name())
	}

	t.Logf("seeded test data for %s (total %d files)", t.Name(), len(filePaths))
	return dirPath, filePaths
}

package helpers

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	mutex   = sync.Mutex{}
	portInc = 42067
)

func getNextPort() int {
	mutex.Lock()
	defer mutex.Unlock()

	portInc++
	return portInc
}

func keyValueToEnv(k, v string) string {
	return fmt.Sprintf("%s=%s", k, v)
}

// spawnTheaProc will spawn a new Thea service instance on the host system. The container
// will have it's environment variables set as per the request provided. This function
// will BLOCK until the Thea instance logs indicate it's ready to receive HTTP requests (or,
// if the timeout is exceeded, in which case error is reported via the testing.T).
func spawnTheaProc(t *testing.T, req TheaServiceRequest) *TestService {
	if req.databaseName == "" {
		t.Fatalf("cannot satisfy Thea service request %#v as no databaseName is specified. Implicit fallback to master DB is disallowed, explicit database must be provided", req)
		return nil
	}

	port := getNextPort()
	t.Logf("Spawning Thea process on port %d for request %s\n", port, req)
	databaseName := req.databaseName

	theaCmd := exec.Command("../../.bin/thea", "-config", "../test-config.toml", "-log-level", "VERBOSE")
	theaCmd.Env = os.Environ()
	for k, v := range req.environmentVariables {
		theaCmd.Env = append(theaCmd.Env, keyValueToEnv(k, v))
	}
	theaCmd.Env = append(theaCmd.Env, keyValueToEnv("API_HOST_ADDR", fmt.Sprintf("0.0.0.0:%d", port)))
	theaCmd.Env = append(theaCmd.Env, keyValueToEnv("DB_NAME", databaseName))

	// If no ingest directory was specified, then specify one automatically
	if _, ok := req.environmentVariables["INGEST_DIR"]; !ok {
		theaCmd.Env = append(theaCmd.Env, keyValueToEnv("INGEST_DIR", t.TempDir()))
	}

	stdout, err := theaCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to execute Thea instance: could not establish stdout pipe: %s", err)
		return nil
	}
	stderr, err := theaCmd.StderrPipe()
	if err != nil {
		t.Fatalf("failed to execute Thea instance: could not establish stdout pipe: %s", err)
		return nil
	}

	err = theaCmd.Start()
	if err != nil {
		t.Fatalf("failed to execute Thea instance: could not run cmd: %s", err)
		return nil
	}

	t.Logf("Thea process started (PID %d)", theaCmd.Process.Pid)
	cleanup := func(t *testing.T) {
		t.Logf("Killing Thea process (PID %d)...", theaCmd.Process.Pid)
		if err := theaCmd.Process.Kill(); err != nil {
			t.Logf("[WARNING] failed to cleanup Thea instance: sending process kill failed: %s", err)
		}

		t.Log("Waiting for Thea process to finish...")
		_ = theaCmd.Wait()
	}

	isReady := make(chan struct{})
	go func() {
		defer close(isReady)

		alreadySeen := false
		out := io.MultiReader(stdout, stderr)
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			text := scanner.Text()
			log.Printf("[Thea pid=%d port=%d db=%s] -> %s", theaCmd.Process.Pid, port, databaseName, text)

			if !alreadySeen && strings.Contains(text, "Thea services spawned") {
				isReady <- struct{}{}
			}
		}

		fmt.Printf("Thea process (%d) for (%s) has closed it's output pipes\n", theaCmd.Process.Pid, req)
	}()

	t.Logf("Waiting for Thea process to become healthy (5s timeout)...")
	select {
	case _, ok := <-isReady:
		if !ok {
			defer cleanup(t)

			t.Fatalf("failed to provision Thea instance: service closed prematurely")
			return nil
		}

		t.Logf("Thea process healthy!")
		return &TestService{Port: port, DatabaseName: databaseName, cleanup: cleanup}
	case <-time.NewTimer(5 * time.Second).C:
		defer cleanup(t)

		t.Fatalf("failed to provision Thea instance: service did not become healthy before timeout")
		return nil
	}
}

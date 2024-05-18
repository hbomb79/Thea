package helpers

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"
)

var (
	mutex   = sync.Mutex{}
	portInc = 42067

	// shouldOutputTheaLogs controls whether the logs from the spawned Thea services are
	// logged via the testing.T.
	shouldOutputTheaLogs = os.Getenv("OUTPUT_THEA_LOGS") != ""
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
//
//nolint:funlen
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

	// If the TMDB API key was not specified manually AND it's
	// not present in the environment, then raise a failure.
	if _, ok := req.environmentVariables["OMDB_API_KEY"]; !ok {
		if _, found := os.LookupEnv("OMDB_API_KEY"); !found {
			t.Fatalf("Request %s from %s did NOT specify a TMDB API key and there is not one present in the environment!"+
				"To suppress, use .WithNoTMDBKey, or provide a key using .WithTMDBKey or the OS environment",
				req, t.Name())
		}
	}

	theaCmd.Env = append(theaCmd.Env, keyValueToEnv("API_HOST_ADDR", fmt.Sprintf("0.0.0.0:%d", port)))
	theaCmd.Env = append(theaCmd.Env, keyValueToEnv("DB_NAME", databaseName))

	// If no ingest directory was specified, then specify one automatically
	if req.ingestDirectory == "" {
		req.ingestDirectory = t.TempDir()
	}
	theaCmd.Env = append(theaCmd.Env, keyValueToEnv("INGEST_DIR", req.ingestDirectory))

	stdout, err := theaCmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to provision Thea instance: could not establish stdout pipe: %s", err)
		return nil
	}
	stderr, err := theaCmd.StderrPipe()
	if err != nil {
		t.Fatalf("failed to provision Thea instance: could not establish stderr pipe: %s", err)
		return nil
	}

	err = theaCmd.Start()
	if err != nil {
		t.Fatalf("failed to provision Thea instance: could not start process: %s", err)
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

		if t.Failed() {
			t.Log("HINT: supply the 'OUTPUT_THEA_LOGS' environment variable to see the logs from the spawned Thea instance")
		}
	}

	if shouldOutputTheaLogs {
		go func() {
			scanner := bufio.NewScanner(io.MultiReader(stdout, stderr))
			for scanner.Scan() {
				text := scanner.Text()
				log.Printf("[Thea pid=%d port=%d db=%s] -> %s", theaCmd.Process.Pid, port, databaseName, text)
			}

			fmt.Printf("Thea process (%d) for (%s) has closed it's output pipes\n", theaCmd.Process.Pid, req)
		}()
	} else if databaseName != MasterDBName {
		t.Logf("Not outputting Thea process logs for %s; 'OUTPUT_THEA_LOGS' env var not set", t.Name())
	}

	srv := &TestService{Port: port, DatabaseName: databaseName, cleanup: cleanup}
	if err := srv.waitForHealthy(t, 100*time.Millisecond, 5*time.Second); err != nil {
		defer cleanup(t)
		t.Fatalf("failed to provision Thea instance: service did not become healthy before timeout (last error %+v)", err)
		return nil
	}

	t.Logf("Thea process (pid %d, port %d) became healthy", theaCmd.Process.Pid, port)
	return srv
}

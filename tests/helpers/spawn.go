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

const (
	EnvDBName                 = "DB_NAME"
	EnvIngestDir              = "INGEST_DIR"
	EnvDefaultOutputDir       = "FORMAT_DEFAULT_OUTPUT_DIR"
	EnvAPIHostAddr            = "API_HOST_ADDR"
	EnvTMDBKey                = "TMDB_API_KEY"
	EnvIngestModtimeThreshold = "INGEST_MODTIME_THRESHOLD_SECONDS"
)

// spawnTheaProc will spawn a new Thea service instance on the host system. The container
// will have it's environment variables set as per the request provided. This function
// will BLOCK until the Thea instance logs indicate it's ready to receive HTTP requests (or,
// if the timeout is exceeded, in which case error is reported via the testing.T).
//
//nolint:funlen
func spawnTheaProc(t *testing.T, req TheaServiceRequest) *TestService {
	port := getNextPort()
	t.Logf("Spawning Thea process on port %d for request %s\n", port, req)

	databaseName, ok := req.environmentVariables[EnvDBName]
	if !ok || databaseName == "" {
		t.Fatalf("cannot satisfy Thea service request %#v as no databaseName is specified. Implicit fallback to master DB is disallowed, explicit database must be provided", req)
		return nil
	}

	// Set defaults for certain env vars
	if _, ok := req.environmentVariables[EnvIngestDir]; !ok {
		req.environmentVariables[EnvIngestDir] = t.TempDir()
	}
	if _, ok := req.environmentVariables[EnvAPIHostAddr]; !ok {
		req.environmentVariables[EnvAPIHostAddr] = fmt.Sprintf("0.0.0.0:%d", port)
	}
	if _, ok := req.environmentVariables[EnvIngestModtimeThreshold]; !ok {
		req.environmentVariables[EnvIngestModtimeThreshold] = "0"
	}

	if req.requiresTMDB {
		_, man := req.environmentVariables[EnvTMDBKey]
		_, fromEnv := os.LookupEnv(EnvTMDBKey)
		if !man && !fromEnv {
			t.Fatalf("ABORTING: Test '%s' has marked itself as requiring access to TMDB, however no TMDB key was found in the environment (env key TMDB_API_KEY)!\n\n"+
				"**HINT**: you can specify the TMDB_API_KEY env var using .env file in the project root or in the 'tests/' directory.\n\n", t.Name())
		}
	}

	theaCmd := exec.Command("../../.bin/thea", "-config", "../test-config.toml", "-log-level", "VERBOSE")

	// Populate command environment with the variables stored inside the service request
	theaCmd.Env = os.Environ()
	for k, v := range req.environmentVariables {
		theaCmd.Env = append(theaCmd.Env, keyValueToEnv(k, v))
	}

	// // If the TMDB API key was not specified manually AND it's
	// // not present in the environment, then raise a failure.
	// if _, ok := req.environmentVariables[EnvTMDBKey]; !ok {
	// 	if _, found := os.LookupEnv(EnvTMDBKey); !found {
	// 		t.Fatalf("Request %s from %s did NOT specify a TMDB API key and there is not one present in the environment!"+
	// 			"To suppress, use .WithNoTMDBKey, or provide a key using .WithTMDBKey or the OS environment",
	// 			req, t.Name())
	// 	}
	// }

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

		if t.Failed() && !shouldOutputTheaLogs {
			t.Log("\n**HINT: Supply the 'OUTPUT_THEA_LOGS' environment variable to see the logs from spawned Thea instances")
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

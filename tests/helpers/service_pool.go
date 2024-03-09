package helpers

import (
	"fmt"
	"sync"
	"testing"
)

type TestServicePool struct {
	*sync.Mutex
	databaseManager *databaseManager
	services        map[string]*TestService
	counts          map[string]int
}

func newTestServicePool() *TestServicePool {
	return &TestServicePool{
		Mutex:           &sync.Mutex{},
		databaseManager: newDatabaseManager(MasterDBName),
		services:        make(map[string]*TestService),
		counts:          make(map[string]int),
	}
}

var (
	servicePool           *TestServicePool = newTestServicePool()
	defaultServiceRequest                  = NewTheaServiceRequest().WithDatabaseName("integration_test")
)

func RequireDefaultThea(t *testing.T) *TestService {
	return servicePool.requireThea(t, defaultServiceRequest)
}

func RequireThea(t *testing.T, request TheaServiceRequest) *TestService {
	return servicePool.requireThea(t, request)
}

// RequireThea will return a TestService back to the caller based on the request provided.
// If the request matches a previously seen request (note that the environment variables inside
// the request are NOT considered when checking for matching requests) then an existing TestService
// may be returned to the caller. If no existing service can satisfy the request, then a new instance
// of Thea will be started inside of a Docker container, pointing to a new database (if specified), and
// running on a unique port number. Cleanup of services is automatic via the testing.T Cleanup functionality.
func (pool *TestServicePool) requireThea(t *testing.T, request TheaServiceRequest) *TestService {
	pool.Lock()
	defer pool.Unlock()

	t.Logf("Test %s requesting Thea service: %s", t.Name(), request)

	srv := pool.getOrCreate(t, request)
	pool.services[request.Key()] = srv
	pool.counts[request.Key()]++

	t.Cleanup(func() { pool.markComplete(t, request) })

	return srv
}

func (pool *TestServicePool) markComplete(t *testing.T, request TheaServiceRequest) {
	pool.Lock()
	defer pool.Unlock()

	reqKey := request.Key()
	pool.counts[reqKey]--

	t.Logf("Test %s finished using Thea service (for request %s)\n", t.Name(), request)
	if pool.counts[reqKey] == 0 {
		t.Logf("All consumers have finished using Thea service (for request %s), shutting down...\n", request)
		// Clear groups and teardown service
		delete(pool.counts, reqKey)
		if serv, ok := pool.services[reqKey]; ok {
			serv.cleanup(t)
			delete(pool.services, reqKey)
		} else {
			t.Logf("[WARNING] Service associated with request %s not found, but it was still being tracked...\n", request)
		}

		if len(pool.services) == 0 {
			t.Log("No services provisioned, cleaning up Postgres container...\n")
			pool.databaseManager.disconnect(t)
		}
	}
}

// getOrCreate will either return an existing Thea service which uses
// the database name specified, or will spawn a new instance of the service
// and provision the database specified.
func (pool *TestServicePool) getOrCreate(t *testing.T, request TheaServiceRequest) *TestService {
	if existing, ok := pool.services[request.Key()]; ok {
		t.Logf("Request '%s' satisfiable by existing service %s", request, existing)
		return existing
	}

	t.Logf("Request for Thea service '%s' has NO matching existing service. Spawning...", request)
	pool.databaseManager.provisionDB(t, request.databaseName)
	return spawnTheaProc(t, request)
	// return spawnThea(t, request)
}

// TheaServiceRequest encapsulates information required to
// request a Thea service from the service pool.
type TheaServiceRequest struct {
	// databaseName defines the name of the PostgreSQL database
	// which this Thea service is expected to connect to. Provisioning
	// of this database will be handled automatically if needed.
	databaseName string

	// ingestDirectory defines an optional directory on the
	// file system which will be set as the ingest directory
	// for the spawned service instance.
	ingestDirectory string

	// environmentVariables can optionally be provided to
	// the request to augment the mandatory API_HOST_ADDR and DB_NAME
	// values that are provided. Note that overriding these values
	// inside of the environmentVariables will have no effect.
	environmentVariables map[string]string
}

func NewTheaServiceRequest() TheaServiceRequest {
	return TheaServiceRequest{
		databaseName:         MasterDBName,
		ingestDirectory:      "",
		environmentVariables: make(map[string]string, 0),
	}
}

func (req TheaServiceRequest) Key() string {
	return fmt.Sprintf("thea-%s-%s", req.databaseName, req.ingestDirectory)
}

func (req TheaServiceRequest) String() string {
	return fmt.Sprintf("Request{db=%s ingestDir=%s}", req.databaseName, req.ingestDirectory)
}

func (req TheaServiceRequest) WithDatabaseName(databaseName string) TheaServiceRequest {
	req.databaseName = databaseName
	return req
}

func (req TheaServiceRequest) WithIngestDirectory(ingestPath string) TheaServiceRequest {
	req.ingestDirectory = ingestPath
	return req
}

func (req TheaServiceRequest) WithEnvironmentVariable(key, value string) TheaServiceRequest {
	req.environmentVariables[key] = value
	return req
}

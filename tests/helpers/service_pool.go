package helpers

import (
	"fmt"
	"sync"
	"testing"
)

type TestServicePool struct {
	*sync.Mutex
	services map[string]*TestService
	counts   map[string]int
}

func (pool *TestServicePool) RequireThea(t *testing.T, databaseName string) *TestService {
	fmt.Printf("Test %s is requesting a Thea instance with DB %s...\n", t.Name(), databaseName)

	service := pool.GetOrCreate(databaseName)
	pool.markInUse(t, databaseName)
	return service
}

func (pool *TestServicePool) markInUse(t *testing.T, databaseName string) {
	pool.Lock()
	defer pool.Unlock()
	pool.counts[databaseName]++
	t.Cleanup(func() { pool.markComplete(t, databaseName) })
}

func (pool *TestServicePool) markComplete(t *testing.T, databaseName string) {
	pool.Lock()
	defer pool.Unlock()

	pool.counts[databaseName]--
	fmt.Printf("Test %s finished with Thea service (for DB %s)\n", t.Name(), databaseName)
	if pool.counts[databaseName] == 0 {
		fmt.Printf("All consumers have finished using Thea service (for DB %s), shutting down...\n", databaseName)
		// Clear groups and teardown service
		delete(pool.counts, databaseName)
		if serv, ok := pool.services[databaseName]; ok {
			serv.cleanup()
			delete(pool.services, databaseName)
		} else {
			fmt.Printf("[WARNING] Service for DB %s not found, but it's WaitGroup was still being tracked...\n", databaseName)
		}

		if len(pool.services) == 0 {
			fmt.Printf("No services provisioned, cleaning up Postgres container...\n")
			dbManager.cleanup()
		}
	}
}

// GetOrCreate will either return an existing Thea service which uses
// the database name specified, or will spawn a new instance of the service
// and provision the database specified.
func (pool *TestServicePool) GetOrCreate(databaseName string) *TestService {
	pool.Lock()
	defer pool.Unlock()

	if existing, ok := pool.services[databaseName]; ok {
		fmt.Printf("Request for Thea service with DB '%s' satisfiable by existing service\n", databaseName)
		return existing
	}

	fmt.Printf("Request for Thea service with DB '%s' has NO matching existing service. Spawning...\n", databaseName)
	service := SpawnTheaManualCleanup(databaseName)
	pool.services[databaseName] = service

	return service
}

func NewTestServicePool() *TestServicePool {
	return &TestServicePool{
		Mutex:    &sync.Mutex{},
		services: make(map[string]*TestService),
		counts:   make(map[string]int),
	}
}

var ServicePool *TestServicePool

func init() {
	ServicePool = NewTestServicePool()
}

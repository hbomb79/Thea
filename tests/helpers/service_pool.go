package helpers

import (
	"fmt"
	"sync"
	"testing"
)

type TestServicePool struct {
	*sync.Mutex
	services map[string]*TestService
	groups   map[string]*sync.WaitGroup
}

func (pool *TestServicePool) RequireThea(t *testing.T, databaseName string) *TestService {
	fmt.Printf("Test %s is requesting a Thea instance with DB %s...\n", t.Name(), databaseName)
	if existingWg, ok := pool.groups[databaseName]; ok {
		existingWg.Add(1)
		t.Cleanup(func() { existingWg.Done() })
	} else {
		wg := &sync.WaitGroup{}
		wg.Add(1)
		t.Cleanup(func() { wg.Done() })
		pool.groups[databaseName] = wg

		// Start a thread to monitor when this service is finished with
		go func() {
			// Wait for all consumers (tests) to be done
			wg.Wait()

			// Gain exclusive access to the pool
			pool.Lock()
			defer pool.Unlock()

			// Clear groups and teardown service
			delete(pool.groups, databaseName)
			if serv, ok := pool.services[databaseName]; ok {
				serv.cleanup()
				delete(pool.services, databaseName)
			}
		}()
	}

	service := pool.GetOrCreate(databaseName)
	return service
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
		groups:   make(map[string]*sync.WaitGroup),
	}
}

var ServicePool *TestServicePool

func init() {
	ServicePool = NewTestServicePool()
}

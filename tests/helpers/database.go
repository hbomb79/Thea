package helpers

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	SQLDialect          = "postgres"
	SQLConnectionString = "host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Pacific/Auckland"
	Host                = "0.0.0.0"
	User                = "postgres"
	Password            = "postgres"
	MasterDBName        = "THEA_DB"
	Port                = "5432"
)

// databaseManager is an internal test helper which facilitates
// the templating of a single 'master' database in a shared postgresql
// docker instance. This allows tests to use individual databases without
// needing to create multiple instances of docker. This manager will:
//   - automatically spawn the container,
//   - migrate the database (using an emphemeral Thea instance),
//   - mark the master database as a template, and,
//   - facilitate provisioning of new databases based off that master database.
type databaseManager struct {
	*sync.Mutex
	masterDatabaseName string
	pgContainer        testcontainers.Container
	connection         *sql.DB
}

func newDatabaseManager(databaseName string) *databaseManager {
	return &databaseManager{
		Mutex:              &sync.Mutex{},
		masterDatabaseName: databaseName,
	}
}

func (manager *databaseManager) provisionDB(t *testing.T, databaseName string) {
	manager.Lock()
	defer manager.Unlock()

	if databaseName == MasterDBName {
		t.Logf("WARNING: ignoring request to provision database '%s' as this DB is the master database, and cannot be provisioned", databaseName)
		return
	}

	if manager.connection == nil {
		t.Log("Database provisioning request received but manager not started yet. Initializing database management...")
		manager.connect(t)
		manager.markMasterDB(t)
		t.Log("Database management initialised!")
	}

	_, err := manager.connection.Exec(fmt.Sprintf(`CREATE DATABASE "%s" TEMPLATE "%s"`, databaseName, manager.masterDatabaseName))
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			if pqErr.Code == "42P04" {
				t.Logf("Database '%s' already provisioned. Reusing database", databaseName)
				return
			}
		}

		t.Fatalf("failed to create provision database '%s' based on template database '%s': (%T) %s", databaseName, manager.masterDatabaseName, err, err)
	}
}

func (manager *databaseManager) connect(t *testing.T) {
	if manager.connection != nil {
		t.Log("WARNING: ignoring request to connect database manager, connection already open")
		return
	}

	if manager.pgContainer == nil {
		manager.spawnPostgres(t)
	} else if !manager.pgContainer.IsRunning() {
		t.Fatalf("failed to connect database manager, container exists but not running")
	}

	// Connect to the database, mark the current DB as the master template.
	dsn := fmt.Sprintf(SQLConnectionString, Host, User, Password, MasterDBName, Port)
	db, err := sql.Open(SQLDialect, dsn)
	if err != nil {
		t.Fatalf("failed to open postgres connection: %s", err)
	}

	for attempt := range 3 {
		err := db.Ping()
		if err != nil {
			if attempt == 3 {
				t.Fatalf("all database connection attempts FAILED")
			} else {
				fmt.Printf("DB connection attempt (%v/5) failed... Retrying in 3s\n", attempt)
				time.Sleep(3 * time.Second)
				continue
			}
		}

		break
	}

	t.Log("Database connection established!")
	manager.connection = db
}

func (manager *databaseManager) markMasterDB(t *testing.T) {
	if manager.connection == nil {
		t.Fatalf("cannot mark master database as template: db connection not established")
		return
	}

	t.Log("Spawning Thea instance to migrate master database...")
	thea := spawnTheaProc(t, NewTheaServiceRequest().WithDatabaseName(manager.masterDatabaseName))
	t.Log("Master DB migrated, closing Thea...")
	thea.cleanup(t)
	t.Log("Thea closed, marking master database as template...")

	if _, err := manager.connection.Exec(fmt.Sprintf(`ALTER DATABASE "%s" WITH is_template TRUE`, manager.masterDatabaseName)); err != nil {
		t.Fatalf("failed to mark master database (%s) as template: %s", manager.masterDatabaseName, err)
	}
}

func (manager *databaseManager) spawnPostgres(t *testing.T) {
	if manager.pgContainer != nil && manager.pgContainer.IsRunning() {
		t.Log("WARNING: ignoring request to spawn PG container, container already running")
		return
	}

	postgresC, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("docker.io/postgres:14.1-alpine"),
		postgres.WithDatabase(MasterDBName),
		postgres.WithUsername(User),
		postgres.WithPassword(Password),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(5*time.Second)),
		testcontainers.WithHostConfigModifier(func(hostConfig *container.HostConfig) { hostConfig.NetworkMode = "host" }),
	)
	if err != nil {
		t.Fatalf("failed to start container: %s", err)
		return
	}

	manager.pgContainer = postgresC
}

func (manager *databaseManager) teardownPostgres(t *testing.T) {
	if manager.pgContainer == nil || !manager.pgContainer.IsRunning() {
		t.Logf("WARNING: ignoring request to teardown postgres container, container not running")
	}

	t.Log("Tearing down Postgres container...")
	timeout := 5 * time.Second
	if err := manager.pgContainer.Stop(ctx, &timeout); err != nil {
		t.Logf("WARNING: failed to stop Postgres container: %s", err)
	}
}

func (manager *databaseManager) disconnect(t *testing.T) {
	manager.Lock()
	defer manager.Unlock()

	t.Logf("Disconnecting database management...")
	if manager.connection != nil {
		_ = manager.connection.Close()
		manager.connection = nil
	}

	if manager.pgContainer.IsRunning() {
		manager.teardownPostgres(t)
		manager.pgContainer = nil
	}
}

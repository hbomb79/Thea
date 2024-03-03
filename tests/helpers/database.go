package helpers

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/lib/pq"
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

var dbManager = DatabaseTemplateManager{}

func init() {
	fmt.Printf("initialising postgres schema management")
	if err := dbManager.InitialiseMasterSchema(); err != nil {
		panic(err)
	}
}

type DatabaseTemplateManager struct {
	masterTemplate string
	cleanup        func()
	dbConn         *sql.DB
}

// InitialiseMasterSchema should be called before any
// integration tests run. It is responsible for spawning the
// global postgres instance, and a temporary Thea instance which
// will migrate the schema of the database to match the expected
// state, before the database is marked as a template. After this, the
// Thea instance is terminated, and all future integration tests
// should make a copy of this templated database for each test.
func (manager *DatabaseTemplateManager) InitialiseMasterSchema() error {
	manager.cleanup = SpawnPostgres()

	service := SpawnTheaManualCleanup(MasterDBName)
	service.cleanup() // Thea has started up and migrated the schema for us, we don't need it anymore

	// Connect to the database, mark the current DB as the master template.
	dsn := fmt.Sprintf(SQLConnectionString, Host, User, Password, MasterDBName, Port)
	db, err := sql.Open(SQLDialect, dsn)
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %w", err)
	}

	attempt := 1
	for {
		err := db.Ping()
		if err != nil {
			if attempt >= 3 {
				return errors.New("all database connection attempts FAILED")
			} else {
				fmt.Printf("DB connection attempt (%v/5) failed... Retrying in 3s\n", attempt)
				attempt++
				time.Sleep(3 * time.Second)
				continue
			}
		}

		break
	}

	fmt.Println("Database connection established!")

	// Mark the current database as the master template
	_, err = db.Exec(fmt.Sprintf(`ALTER DATABASE "%s" WITH is_template TRUE`, MasterDBName))
	if err != nil {
		return fmt.Errorf("failed to mark master database as template: %w", err)
	}

	fmt.Printf("Setting DB connection %T and master DB %s\n", db, MasterDBName)
	manager.dbConn = db
	manager.masterTemplate = MasterDBName
	return nil
}

// SeedNewDatabase allows a specific test (or a selection of subtests) to
// create a new database within the postgres instance based off of a master template.
func (manager *DatabaseTemplateManager) SeedNewDatabase(databaseName string) {
	if databaseName == MasterDBName {
		return
	}

	_, err := manager.dbConn.Exec(fmt.Sprintf(`CREATE DATABASE "%s" TEMPLATE "%s"`, databaseName, MasterDBName))
	if err != nil {
		fmt.Printf("Failed to create new database %s based on template %s: %v", databaseName, MasterDBName, err)
	}
}

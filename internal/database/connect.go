package database

import (
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

const (
	SqlDialect          = "postgres"
	SqlConnectionString = "host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Pacific/Auckland"
)

var (
	//go:embed migrations/*.sql
	migrations embed.FS

	dbLogger = logger.Get("DB")
)

type (
	Manager interface {
		Connect(DatabaseConfig) error
		GetGoquDb() *sqlx.DB
		RegisterModels(...any)
	}

	manager struct {
		rawDb  *sql.DB
		db     *sqlx.DB
		models []interface{}
	}
)

func New() *manager {
	return &manager{models: make([]any, 0)}
}

func (db *manager) Connect(config DatabaseConfig) error {
	sql, err := sql.Open(SqlDialect, fmt.Sprintf(SqlConnectionString, config.Host, config.User, config.Password, config.Name, config.Port))
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %s", err.Error())
	}
	defer sql.Close()

	attempt := 1
	time.Sleep(time.Second * 2)
	for {
		err := sql.Ping()
		if err != nil {
			if attempt >= 5 {
				dbLogger.Emit(logger.ERROR, "All attempts FAILED!\n")
				return err
			} else {
				dbLogger.Emit(logger.WARNING, "Attempt (%v/5) failed... Retrying in 3s\n", attempt)
				attempt++
				time.Sleep(time.Second * 3)
				continue
			}
		}

		db.rawDb = sql
		db.db = sqlx.NewDb(sql, SqlDialect)

		break
	}

	if err := db.ExecuteMigrations(); err != nil {
		return err
	}

	dbLogger.Emit(logger.SUCCESS, "Database connection complete!\n")
	return nil
}

// ExecuteMigrations uses the comp-time embedded SQL migrations (found in the 'migrations'
// dir in this package) and runs them against the current DB instance.
//
// Note that this method must only be called following a successful DB connection. If the connection
// is not yet established, then this method panics.
func (db *manager) ExecuteMigrations() error {
	rawDb := db.rawDb
	if rawDb == nil {
		return fmt.Errorf("cannot execute migrations when DB manager has not yet connected")
	}

	goose.SetBaseFS(migrations)
	goose.SetLogger(dbLogger)
	if err := goose.SetDialect(SqlDialect); err != nil {
		return fmt.Errorf("failed to set dialect for DB migration: %s", err.Error())
	}

	dbLogger.Emit(logger.INFO, "Checking for pending DB migrations...\n")
	goose.Status(rawDb, "migrations")
	if err := goose.Up(rawDb, "migrations"); err != nil {
		return fmt.Errorf("failed to migrate DB: %s", err.Error())
	}

	dbLogger.Emit(logger.SUCCESS, "DB Goose migration compelte!\n")
	return nil
}

// GetInstances returns the Goqu database connection if
// one has been opened using 'Connect'. Otherwise, nil is returned
func (db *manager) GetGoquDb() *sqlx.DB {
	return db.db
}

func (db *manager) RegisterModels(models ...any) {
	if db.db != nil {
		panic("cannot register models to a database server that is already connected")
	}

	dbLogger.Emit(logger.DEBUG, "Registering DB models %#v\n", models)
	db.models = append(db.models, models...)
}

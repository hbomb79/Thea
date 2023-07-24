package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/hbomb79/Thea/pkg/logger"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
	sqldblogger "github.com/simukti/sqldb-logger"
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
	SqlLogger struct {
		logger logger.Logger
	}

	Manager interface {
		Connect(DatabaseConfig) error
		GetSqlxDb() *sqlx.DB
		WrapTx(func(*sqlx.Tx) error) error
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
	dsn := fmt.Sprintf(SqlConnectionString, config.Host, config.User, config.Password, config.Name, config.Port)
	sql, err := sql.Open(SqlDialect, dsn)
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %s", err.Error())
	}

	sql = sqldblogger.OpenDriver(dsn, sql.Driver(), &SqlLogger{dbLogger})

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
func (db *manager) GetSqlxDb() *sqlx.DB {
	return db.db
}

// WrapTx is a convinience method around the top-level WrapTx, which simply
// uses the managers DB instance as the first argument.
func (db *manager) WrapTx(f func(tx *sqlx.Tx) error) error {
	if db.db == nil {
		return errors.New("DB manager has not yet connected")
	}

	return WrapTx(db.db, f)
}

func (db *manager) RegisterModels(models ...any) {
	if db.db != nil {
		panic("cannot register models to a database server that is already connected")
	}

	dbLogger.Emit(logger.DEBUG, "Registering DB models %#v\n", models)
	db.models = append(db.models, models...)
}

func (l *SqlLogger) Log(_ context.Context, level sqldblogger.Level, msg string, data map[string]any) {
	template := "%s - %v\n"
	switch level {
	case sqldblogger.LevelTrace:
		l.logger.Verbosef(template, msg, data)
	case sqldblogger.LevelDebug, sqldblogger.LevelInfo:
		duration := data["duration"]
		query, ok := data["query"]
		if ok {
			l.logger.Infof("%s [%.2fms] -- %s\n", msg, duration, query)
		} else {
			l.logger.Infof("%s [%.2fms]\n", msg, duration)
		}
	case sqldblogger.LevelError:
		l.logger.Errorf(template, msg, data)
	}
}

// WrapTx starts a transaction against the provided DB, and then calls the user
// provided function. If this function errors, the transaction is rolled back - otherwise
// the transaction is committed.
func WrapTx(db *sqlx.DB, f func(tx *sqlx.Tx) error) error {
	tx, err := db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := f(tx); err != nil {
		dbLogger.Errorf("Transaction failed... rolling back. Error: %s\n", err.Error())
		return err
	}

	return tx.Commit()
}

// InExec is a convinience method which combines sqlx's `In` method
// and the `Exec` of the output query. Rebinding of the
// query is handled automatically, and errors resulting from
// either step will be returned.
func InExec(db *sqlx.Tx, query string, arg any) error {
	if q, a, e := sqlx.In(query, arg); e == nil {
		if _, err := db.Exec(db.Rebind(q), a...); err != nil {
			return err
		}
	} else {
		return e
	}

	return nil
}

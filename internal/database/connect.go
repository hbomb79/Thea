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
	}
	// Queryable includes all methods shared by sqlx.DB and sqlx.Tx, allowing
	// either type to be used interchangeably.
	Queryable interface {
		sqlx.Ext
		sqlx.ExecerContext
		sqlx.PreparerContext
		sqlx.QueryerContext
		sqlx.Preparer

		GetContext(context.Context, interface{}, string, ...interface{}) error
		SelectContext(context.Context, interface{}, string, ...interface{}) error
		Get(interface{}, string, ...interface{}) error
		MustExecContext(context.Context, string, ...interface{}) sql.Result
		PreparexContext(context.Context, string) (*sqlx.Stmt, error)
		QueryRowContext(context.Context, string, ...interface{}) *sql.Row
		Select(interface{}, string, ...interface{}) error
		QueryRow(string, ...interface{}) *sql.Row
		PrepareNamedContext(context.Context, string) (*sqlx.NamedStmt, error)
		PrepareNamed(string) (*sqlx.NamedStmt, error)
		Preparex(string) (*sqlx.Stmt, error)
		NamedExec(string, interface{}) (sql.Result, error)
		NamedExecContext(context.Context, string, interface{}) (sql.Result, error)
		MustExec(string, ...interface{}) sql.Result
		NamedQuery(string, interface{}) (*sqlx.Rows, error)
	}

	manager struct {
		rawDb *sql.DB
		db    *sqlx.DB
	}
)

func New() *manager {
	return &manager{}
}

func (db *manager) Connect(config DatabaseConfig) error {
	dsn := fmt.Sprintf(SqlConnectionString, config.Host, config.User, config.Password, config.Name, config.Port)
	sql, err := sql.Open(SqlDialect, dsn)
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %w", err)
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
		return fmt.Errorf("failed to set dialect for DB migration: %w", err)
	}

	dbLogger.Emit(logger.INFO, "Checking for pending DB migrations...\n")
	goose.Status(rawDb, "migrations")
	if err := goose.Up(rawDb, "migrations"); err != nil {
		return fmt.Errorf("failed to migrate DB: %w", err)
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

func (l *SqlLogger) Log(_ context.Context, level sqldblogger.Level, msg string, data map[string]any) {
	template := "%s - %v\n"
	switch level {
	case sqldblogger.LevelTrace:
		l.logger.Verbosef(template, msg, data)
	case sqldblogger.LevelDebug, sqldblogger.LevelInfo:
		duration := data["duration"]
		query, ok := data["query"]
		if ok {
			l.logger.Debugf("%s [%.2fms] -- %s\n", msg, duration, query)
		} else {
			l.logger.Debugf("%s [%.2fms]\n", msg, duration)
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
		dbLogger.Errorf("Transaction failed... rolling back. Error: %v\n", err)
		return fmt.Errorf("wrapped DB transaction failed: %w", err)
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

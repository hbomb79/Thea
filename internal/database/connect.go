package database

import (
	"fmt"
	"time"

	"github.com/hbomb79/Thea/pkg/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var dbLogger = logger.Get("DB")

/*
 * Thea requires access to a PostgreSQL database to manage relational data - to lower complexity of installation,
 * we provide this database automatically by instantiating it via the Docker SDK. This allows us to spawn and manage the
 * database ourselves, and avoids polluting the users system with a database installation.
 */

type Manager interface {
	Connect(DatabaseConfig) error
	GetInstance() *gorm.DB
	RegisterModels(...any)
}

type manager struct {
	gorm   *gorm.DB
	models []interface{}
}

func New(models ...any) *manager {
	return &manager{models: models}
}

func (db *manager) Connect(config DatabaseConfig) error {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Pacific/Auckland",
		config.Host,
		config.User,
		config.Password,
		config.Name,
		config.Port,
	)

	attempt := 1
	time.Sleep(time.Second * 2)
	for {
		gorm, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
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

		db.gorm = gorm
		break
	}

	dbLogger.Emit(logger.INFO, "GORM database connection established... performing auto-migrations...\n")
	if err := db.gorm.AutoMigrate(db.models...); err != nil {
		return err
	}
	dbLogger.Emit(logger.SUCCESS, "GORM database connection complete!\n")

	return nil
}

// GetInstances returns the GORM database connection if
// one has been opened using 'Connect'. Otherwise, nil is returned
func (db *manager) GetInstance() *gorm.DB {
	return db.gorm
}

func (db *manager) RegisterModels(models ...any) {
	if db.gorm != nil {
		panic("cannot register models to a database server that is already connected")
	}

	dbLogger.Emit(logger.DEBUG, "Registering DB models %#v\n", models)
	db.models = append(db.models, models...)
}

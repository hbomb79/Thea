package pkg

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var dbLogger = Log.GetLogger("DB", CORE)

/*
 * TPA requires access to a PostgreSQL database to manage relational data - to lower complexity of installation,
 * we provide this database automatically by instantiating it via the Docker SDK. This allows us to spawn and manage the
 * database ourselves, and avoids polluting the users system with a database installation.
 */

type DatabaseServer interface {
	Connect(DatabaseConfig) error
	GetInstance() *gorm.DB
	RegisterModel(...interface{})
}

type dbServer struct {
	gorm   *gorm.DB
	models []interface{}
}

var DB DatabaseServer = &dbServer{models: make([]interface{}, 0)}

func (db *dbServer) Connect(config DatabaseConfig) error {
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
			if attempt > 3 {
				dbLogger.Emit(ERROR, "All attempts FAILED!\n")
				return err
			} else {
				dbLogger.Emit(WARNING, "Attempt (%v/3) failed... Retrying in 3s\n", attempt)
				attempt++
				time.Sleep(time.Second * 3)
				continue
			}
		}

		db.gorm = gorm
		break
	}

	dbLogger.Emit(INFO, "GORM database connection established... performing auto-migrations...\n")
	if err := db.gorm.AutoMigrate(db.models...); err != nil {
		return err
	}
	dbLogger.Emit(SUCCESS, "GORM database connection complete!\n")

	return nil
}

func (db *dbServer) GetInstance() *gorm.DB {
	return db.gorm
}

func (db *dbServer) RegisterModel(models ...interface{}) {
	db.models = append(db.models, models...)
}

package pkg

import (
	"fmt"
	"net"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

/*
 * TPA requires access to a PostgreSQL database to manage relational data - to lower complexity of installation,
 * we provide this database automatically by instantiating it via the Docker SDK. This allows us to spawn and manage the
 * database ourselves, and avoids polluting the users system with a database installation.
 */

type DatabaseServer interface {
	Connect(DatabaseConfig) error
	Close() error
}

type dbServer struct {
	gorm *gorm.DB
}

func NewDatabaseServer() DatabaseServer {
	return &dbServer{}
}

func (db *dbServer) Connect(config DatabaseConfig) error {
	db.waitForPortOpen(fmt.Sprintf("%s:%s", config.Host, config.Port))
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Pacific/Auckland",
		config.Host,
		config.User,
		config.Password,
		config.Name,
		config.Port,
	)

	attempt := 1
	for {
		gorm, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		if err != nil {
			if attempt > 3 {
				fmt.Printf("[DB] All attempts FAILED!\n")
				return err
			} else {
				fmt.Printf("[DB] Attempt (%v/3) failed... Retrying in 5s\n", attempt)
				attempt++
				time.Sleep(time.Second * 5)
				continue
			}
		}

		fmt.Printf("[DB] Attempt (%v/3) succeeded! Connection established\n", attempt)
		db.gorm = gorm
		break
	}

	return nil
}

func (db *dbServer) Close() error {
	return nil
}

func (db *dbServer) waitForPortOpen(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, time.Second*10)

	if err != nil {
		fmt.Printf("[DB] Timed out waiting for DB at %s\n", addr)
	} else {
		if conn != nil {
			fmt.Printf("[DB] Successfully pinged database at %s\n", addr)
			conn.Close()

			return nil
		} else {
			fmt.Printf("[DB] Timed out waiting for DB at %s\n", addr)
		}
	}

	return nil
}

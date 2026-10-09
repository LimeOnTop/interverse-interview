package config

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"
)

// DatabasePool bounds this process's connections; MaxOpen includes idle connections.
// Defaults target one replica per service on a shared 3-vCPU / 4-GiB host.
type DatabasePool struct {
	MaxOpen     int
	MaxIdle     int
	MaxIdleTime time.Duration
	MaxLifetime time.Duration
}

func LoadDatabasePool() (DatabasePool, error) {
	pool := DatabasePool{MaxOpen: 12, MaxIdle: 2, MaxIdleTime: 5 * time.Minute, MaxLifetime: 30 * time.Minute}
	for _, setting := range []struct {
		name    string
		target  *int
		minimum int
	}{
		{"DB_MAX_OPEN_CONNS", &pool.MaxOpen, 1},
		{"DB_MAX_IDLE_CONNS", &pool.MaxIdle, 0},
	} {
		if value, ok := os.LookupEnv(setting.name); ok {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < setting.minimum {
				return DatabasePool{}, fmt.Errorf("%s must be an integer >= %d", setting.name, setting.minimum)
			}
			*setting.target = parsed
		}
	}
	if pool.MaxIdle > pool.MaxOpen {
		return DatabasePool{}, fmt.Errorf("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	for _, setting := range []struct {
		name   string
		target *time.Duration
	}{
		{"DB_CONN_MAX_IDLE_TIME", &pool.MaxIdleTime},
		{"DB_CONN_MAX_LIFETIME", &pool.MaxLifetime},
	} {
		if value, ok := os.LookupEnv(setting.name); ok {
			parsed, err := time.ParseDuration(value)
			if err != nil || parsed <= 0 {
				return DatabasePool{}, fmt.Errorf("%s must be a positive duration (for example 5m)", setting.name)
			}
			*setting.target = parsed
		}
	}
	return pool, nil
}

func ConfigureDatabasePool(db *sql.DB) (DatabasePool, error) {
	pool, err := LoadDatabasePool()
	if err != nil {
		return DatabasePool{}, err
	}
	db.SetMaxOpenConns(pool.MaxOpen)
	db.SetMaxIdleConns(pool.MaxIdle)
	db.SetConnMaxIdleTime(pool.MaxIdleTime)
	db.SetConnMaxLifetime(pool.MaxLifetime)
	return pool, nil
}

package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	SQL *sql.DB
}

type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func loadPoolConfig(getenv func(string) string) (PoolConfig, error) {
	parseInt := func(key string, defaultValue int) (int, error) {
		value := strings.TrimSpace(getenv(key))
		if value == "" {
			return defaultValue, nil
		}
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf("%s must be a positive integer", key)
		}
		return parsed, nil
	}
	parseDuration := func(key string, defaultValue time.Duration) (time.Duration, error) {
		value := strings.TrimSpace(getenv(key))
		if value == "" {
			return defaultValue, nil
		}
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			return 0, fmt.Errorf("%s must be a positive duration", key)
		}
		return parsed, nil
	}

	maxOpen, err := parseInt("DB_MAX_OPEN_CONNS", 10)
	if err != nil {
		return PoolConfig{}, err
	}
	maxIdle, err := parseInt("DB_MAX_IDLE_CONNS", 5)
	if err != nil {
		return PoolConfig{}, err
	}
	maxLifetime, err := parseDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute)
	if err != nil {
		return PoolConfig{}, err
	}
	maxIdleTime, err := parseDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute)
	if err != nil {
		return PoolConfig{}, err
	}
	if maxIdle > maxOpen {
		return PoolConfig{}, fmt.Errorf("DB_MAX_IDLE_CONNS must not exceed DB_MAX_OPEN_CONNS")
	}
	return PoolConfig{
		MaxOpenConns:    maxOpen,
		MaxIdleConns:    maxIdle,
		ConnMaxLifetime: maxLifetime,
		ConnMaxIdleTime: maxIdleTime,
	}, nil
}

func New() (*DB, error) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		// example: postgres://user:pass@localhost:5432/thera?sslmode=disable
		return nil, fmt.Errorf("POSTGRES_DSN is not set")
	}
	poolConfig, err := loadPoolConfig(os.Getenv)
	if err != nil {
		return nil, fmt.Errorf("database pool configuration: %w", err)
	}

	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(poolConfig.MaxOpenConns)
	sqlDB.SetMaxIdleConns(poolConfig.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(poolConfig.ConnMaxLifetime)
	sqlDB.SetConnMaxIdleTime(poolConfig.ConnMaxIdleTime)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	log.Println("connected to Postgres")
	return &DB{SQL: sqlDB}, nil
}

package database

import (
	"database/sql"
	"fmt"
	"time"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"

	_ "github.com/lib/pq"
)

// DB представляет подключение к базе данных
type DB struct {
	*sql.DB
}

// Connect создает подключение к базе данных
func Connect(cfg *config.DatabaseConfig, log *logger.Logger) (*DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Настройка пула соединений
	db.SetMaxOpenConns(25)                 // Максимальное количество открытых соединений
	db.SetMaxIdleConns(5)                  // Максимальное количество неактивных соединений
	db.SetConnMaxLifetime(5 * time.Minute) // Максимальное время жизни соединения

	// Проверка подключения
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Successfully connected to database")

	return &DB{DB: db}, nil
}

// Close закрывает подключение к базе данных
func (db *DB) Close() error {
	return db.DB.Close()
}

// Health проверяет состояние базы данных
func (db *DB) Health() error {
	return db.Ping()
}

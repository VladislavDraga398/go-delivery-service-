package database

import (
	"testing"

	"delivery-system/internal/config"
	"delivery-system/internal/logger"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestHealth(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer sqlDB.Close()

	mock.ExpectPing().WillReturnError(nil)

	db := &DB{DB: sqlDB}
	if err := db.Health(); err != nil {
		t.Fatalf("expected health ok, got %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestConnect_Failure(t *testing.T) {
	log := logger.New(&config.LoggerConfig{Level: "error", Format: "json"})
	cfg := &config.DatabaseConfig{Host: "127.0.0.1", Port: "0", User: "u", Password: "p", DBName: "db", SSLMode: "disable"}
	if _, err := Connect(cfg, log); err == nil {
		t.Fatalf("expected connect error")
	}
}

func TestClose(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock error: %v", err)
	}
	mock.ExpectClose()
	db := &DB{DB: sqlDB}
	if err := db.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestClose_Nil(t *testing.T) {
	var db *DB
	if err := db.Close(); err != nil {
		t.Fatalf("expected nil error on nil db close, got %v", err)
	}
}

func TestHealth_Nil(t *testing.T) {
	var db *DB
	if err := db.Health(); err == nil {
		t.Fatalf("expected error for nil db health")
	}
}

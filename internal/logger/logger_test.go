package logger

import (
	"errors"
	"os"
	"testing"

	"delivery-system/internal/config"

	"github.com/sirupsen/logrus"
)

func TestLogger_Defaults(t *testing.T) {
	log := New(&config.LoggerConfig{Level: "info", Format: "json"})
	if log == nil {
		t.Fatalf("logger is nil")
	}
	log.WithField("test", "value").Info("test message")
}

func TestLogger_FileOutput(t *testing.T) {
	tmpfile, err := os.CreateTemp("", "logtest")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	_ = tmpfile.Close()
	defer os.Remove(tmpfile.Name())

	log := New(&config.LoggerConfig{Level: "debug", Format: "text", File: tmpfile.Name()})
	log.Debug("file log")
}

func TestLogger_WithFieldsAndError(t *testing.T) {
	log := New(&config.LoggerConfig{Level: "info", Format: "text"})
	entry := log.WithFields(logrus.Fields{"key": "value"})
	if entry == nil {
		t.Fatalf("entry is nil")
	}
	errEntry := log.WithError(errors.New("fail"))
	if errEntry == nil {
		t.Fatalf("error entry is nil")
	}
}

package main

import (
	"context"
	"github.com/JohnnyConstantin/go_mart/internal/config"
	"github.com/JohnnyConstantin/go_mart/internal/store"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestLoadEnvs(t *testing.T) {
	t.Run("should override config with env vars", func(t *testing.T) {

		// Устанавливаем тестовые переменные окружения
		os.Setenv("RUN_ADDRESS", "test_addr:1234")
		os.Setenv("DATABASE_URI", "test_db_uri")
		os.Setenv("ACCRUAL_SYSTEM_ADDRESS", "test_accrual_addr")

		config.GetConfig()

		assert.Equal(t, "test_addr:1234", config.Config.ServerAddress)
		assert.Equal(t, "test_db_uri", config.Config.DatabaseURL)
		assert.Equal(t, "test_accrual_addr", config.Config.AccrualAddress)
	})
}

func TestInitStorage(t *testing.T) {
	t.Run("should fail with invalid DSN", func(t *testing.T) {

		// Устанавливаем невалидный DSN
		config.Config.DatabaseURL = "invalid_dsn"

		ctx := context.Background()
		db, err := store.OpenDB(ctx, config.Config.DatabaseURL)
		assert.Error(t, err)
		assert.Nil(t, db)
	})
}

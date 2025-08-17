package store

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Migrator Объект, выполняющий миграции для базы данных
type Migrator struct {
	Db  Database
	dir fs.FS // Необходимо для динамической конфигурации директории с миграциями
}

func NewMigrator(db Database, dir fs.FS) *Migrator {
	return &Migrator{Db: db, dir: dir}
}

func (m *Migrator) Migrate(ctx context.Context) error {
	// Читаем файлы миграций
	files, err := fs.ReadDir(m.dir, ".")
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %v", err)
	}

	// Сортируем файлы по порядку миграций
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	// Создаем таблицу для учета выполненных миграций (overkill, но вроде практика хорошая)
	if _, err := m.Db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INT PRIMARY KEY,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Применяем миграции по порядку
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".up.sql") {
			continue
		}

		// Извлекаем номер версии из имени файла
		versionStr := strings.Split(file.Name(), "_")[0]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			return fmt.Errorf("invalid migration version in filename %s: %w", file.Name(), err)
		}

		// Проверяем, не была ли уже применена эта миграция
		var exists bool
		err = m.Db.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)",
			version,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration version %d: %w", version, err)
		}

		if exists {
			continue
		}

		// Читаем SQL-запрос из файла
		sqlBytes, err := fs.ReadFile(m.dir, file.Name())
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file.Name(), err)
		}

		// Выполняем миграцию в транзакции
		tx, err := m.Db.BeginTx(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}
		defer tx.Rollback(ctx)

		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file.Name(), err)
		}

		// Записываем факт выполнения миграции
		if _, err := tx.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1)",
			version,
		); err != nil {
			return fmt.Errorf("failed to record migration version %d: %w", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", version, err)
		}

		fmt.Printf("Applied migration: %s\n", file.Name())
	}

	return nil
}

func (m *Migrator) Rollback(ctx context.Context, version int) error {
	// Находим файл отката для указанной версии
	pattern := fmt.Sprintf("%04d_*.down.sql", version)
	var downFile string
	err := fs.WalkDir(m.dir, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if matched, _ := filepath.Match(pattern, d.Name()); matched {
			downFile = d.Name()
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to find rollback file: %w", err)
	}

	if downFile == "" {
		return fmt.Errorf("rollback file for version %d not found", version)
	}

	// Читаем SQL-запрос из файла
	sqlBytes, err := fs.ReadFile(m.dir, downFile)
	if err != nil {
		return fmt.Errorf("failed to read rollback file %s: %w", downFile, err)
	}

	// Выполняем откат в транзакции
	tx, err := m.Db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("failed to execute rollback %s: %w", downFile, err)
	}

	// Удаляем запись о миграции
	if _, err := tx.Exec(ctx,
		"DELETE FROM schema_migrations WHERE version = $1",
		version,
	); err != nil {
		return fmt.Errorf("failed to remove migration version %d: %w", version, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit rollback: %w", err)
	}

	return nil
}

package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Приходится вшивать миграции в бинарь. Не придумал как иначе для автотестов надежно передать миграции
//
//go:embed migrations/*.sql
var embedMigrations embed.FS

// Migrator Объект, выполняющий миграции для базы данных
type Migrator struct {
	DB  Database
	dir fs.FS
}

func NewMigrator(db Database) *Migrator {
	return &Migrator{DB: db, dir: embedMigrations}
}

func (m *Migrator) Migrate(ctx context.Context) error {
	// Вытаскиваем из embedded папку migrations с файлами внутри
	files, err := fs.ReadDir(m.dir, "migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %w", err)
	}

	// Создание таблицы для учета миграций (наверное overkill, но помогает с автотестами и в целом хорошая практика)
	if _, err := m.DB.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version INT PRIMARY KEY,
            applied_at TIMESTAMP NOT NULL DEFAULT NOW()
        );
    `); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// сортируем
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	// Проходимся по миграциям
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".up.sql") {
			continue
		}

		versionStr := strings.Split(file.Name(), "_")[0]
		version, err := strconv.Atoi(versionStr)
		if err != nil {
			return fmt.Errorf("invalid migration version in filename %s: %w", file.Name(), err)
		}

		// Проверяем выполнялась ли эта миграция
		var exists bool
		err = m.DB.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)",
			version,
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration version %d: %w", version, err)
		}

		if exists {
			fmt.Printf("Migration %d already applied, skipping\n", version)
			continue
		}

		// Вытаскиваем содержимое
		migrationPath := filepath.Join("migrations", file.Name())
		sqlBytes, err := fs.ReadFile(m.dir, migrationPath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", migrationPath, err)
		}

		// Выполнение транзации
		tx, err := m.DB.BeginTx(ctx)
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %w", err)
		}

		defer func() {
			if err != nil {
				tx.Rollback(ctx)
			}
		}()

		// Выполняем миграцию
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w\nSQL: %s",
				file.Name(), err, string(sqlBytes))
		}

		// Фиксируем в бд выполненую миграцию
		if _, err := tx.Exec(ctx,
			"INSERT INTO schema_migrations (version) VALUES ($1)",
			version,
		); err != nil {
			return fmt.Errorf("failed to record migration version %d: %w", version, err)
		}

		// Коммитимся
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("failed to commit migration %d: %w", version, err)
		}

		fmt.Printf("Successfully applied migration: %s\n", file.Name())
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
	tx, err := m.DB.BeginTx(ctx)
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

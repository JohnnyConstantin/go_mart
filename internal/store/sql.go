package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// postgresDB - реализация Database для Postgre
type postgresDB struct {
	pool *pgxpool.Pool
	DB   *sql.DB
}

// postgresTx - реализация Transaction
type postgresTx struct {
	tx pgx.Tx
}

// OpenDB создает новое подключение к БД
func OpenDB(ctx context.Context, dsn string) (Database, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse dsn: %w", err)
	}

	// Создание пула
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create pg pool: %w", err)
	}

	// Проверка подключения
	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctxTimeout); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &postgresDB{pool: pool}, nil
}

// Имплементации методов:

func (db *postgresDB) BeginTx(ctx context.Context) (Transaction, error) {
	tx, err := db.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	return &postgresTx{tx: tx}, nil
}

func (db *postgresDB) Exec(ctx context.Context, query string, args ...interface{}) (int64, error) {
	ct, err := db.pool.Exec(ctx, query, args...)
	return ct.RowsAffected(), err
}

func (db *postgresDB) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	return db.pool.QueryRow(ctx, query, args...)
}

func (db *postgresDB) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	return db.pool.Query(ctx, query, args...)
}

func (db *postgresDB) Close() {
	db.pool.Close()
}

func (t *postgresTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *postgresTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

func (t *postgresTx) BeginTx(ctx context.Context) (Transaction, error) {
	return nil, fmt.Errorf("nested transactions are not supported")
}

func (t *postgresTx) Exec(ctx context.Context, query string, args ...interface{}) (int64, error) {
	ct, err := t.tx.Exec(ctx, query, args...)
	return ct.RowsAffected(), err
}

func (t *postgresTx) QueryRow(ctx context.Context, query string, args ...interface{}) Row {
	return t.tx.QueryRow(ctx, query, args...)
}

func (t *postgresTx) Query(ctx context.Context, query string, args ...interface{}) (Rows, error) {
	return t.tx.Query(ctx, query, args...)
}

func (t *postgresTx) Close() {
	_ = t.Rollback(context.Background())
}

// IsDuplicateKeyError проверяет ошибку на дублирование ключа
func IsDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" // уже известный с прошлых итераций код ошибки
}

// IsForeignKeyError проверяет ошибку нарушения внешнего ключа
func IsForeignKeyError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503" // уже известный с прошлых итераций код ошибки
}

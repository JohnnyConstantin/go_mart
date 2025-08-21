package store

import "context"

// Database - основной интерфейс для работы с БД
type Database interface {
	DBTransaction
	Exec(ctx context.Context, query string, args ...interface{}) (int64, error)
	QueryRow(ctx context.Context, query string, args ...interface{}) Row
	Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
	Close()
}

// DBTransaction - интерфейс для работы с транзакциями
type DBTransaction interface {
	BeginTx(ctx context.Context) (Transaction, error)
}

// Transaction - интерфейс транзакции
type Transaction interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Database
}

// Row - интерфейс для работы с одной строкой результата
type Row interface {
	Scan(dest ...interface{}) error
}

// Rows - интерфейс для работы с набором строк
type Rows interface {
	Close()
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}

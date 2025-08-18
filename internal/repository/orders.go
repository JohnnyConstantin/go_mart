package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/JohnnyConstantin/go_mart/internal/store"
	"io"
	"net/http"
	"time"
)

type OrderRepository struct {
	db store.Database
}

type Order struct {
	ID         int       `json:"-"`
	UserID     string    `json:"-"`
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    float64   `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

func NewOrderRepository(db store.Database) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) CreateOrder(ctx context.Context, order *Order) error {
	// Сначала проверяем существование заказа
	existingOrder, err := r.GetOrderByNumber(ctx, order.Number)
	if err != nil && !errors.Is(err, store.ErrOrderNotFound) {
		return fmt.Errorf("failed to check existing order: %w", err)
	}

	// Если заказ существует и принадлежит другому пользователю
	if existingOrder != nil && existingOrder.UserID != order.UserID {
		return store.ErrOrderForOtherUser
	}

	// Если заказ уже существует у этого пользователя
	if existingOrder != nil && existingOrder.UserID == order.UserID {
		return store.ErrOrderForThisUser
	}

	// Создаем новый заказ
	query := `
        INSERT INTO orders (user_id, number, status, accrual)
        VALUES ($1, $2, $3, $4)
        RETURNING id, uploaded_at
    `

	err = r.db.QueryRow(ctx, query,
		order.UserID,
		order.Number,
		order.Status,
		order.Accrual,
	).Scan(&order.ID, &order.UploadedAt)

	return err
}

func (r *OrderRepository) GetOrderByNumber(ctx context.Context, number string) (*Order, error) {
	query := `
		SELECT id, user_id, number, status, accrual, uploaded_at
		FROM orders
		WHERE number = $1
	`

	var o Order
	err := r.db.QueryRow(ctx, query, number).Scan(
		&o.ID,
		&o.UserID,
		&o.Number,
		&o.Status,
		&o.Accrual,
		&o.UploadedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, store.ErrOrderNotFound
	}

	return &o, err
}

func (r *OrderRepository) GetOrdersForProcessing(ctx context.Context) ([]Order, error) {
	query := `
		SELECT id, user_id, number, status, accrual, uploaded_at
		FROM orders
		WHERE status IN ('NEW', 'PROCESSING', 'REGISTERED')
		ORDER BY uploaded_at ASC
		LIMIT 100
	`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Number, &o.Status, &o.Accrual, &o.UploadedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}

	return orders, nil
}

func (r *OrderRepository) UpdateOrder(ctx context.Context, order *Order) error {
	query := `
		UPDATE orders
		SET status = $1, accrual = $2
		WHERE id = $3
	`
	_, err := r.db.Exec(ctx, query, order.Status, order.Accrual, order.ID)
	return err
}

// ValidateOrderNumber Валидация номера ордера по алгоритму Луна
func ValidateOrderNumber(number string) bool {
	sum := 0
	alternate := false

	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0')
		if alternate {
			digit *= 2
			if digit > 9 {
				digit = (digit % 10) + 1
			}
		}
		sum += digit
		alternate = !alternate
	}

	return sum%10 == 0
}

// ReadOrderNumber Вытаскиваем ордер из запроса
func ReadOrderNumber(r *http.Request) (string, error) {
	body := make([]byte, 100)
	n, err := r.Body.Read(body)
	if err != nil && err != io.EOF {
		return "", err
	}
	return string(body[:n]), nil
}

// GetUserOrders Получение объектов заказов для конкретного пользователя
func (r *OrderRepository) GetUserOrders(ctx context.Context, userID string) ([]Order, error) {
	query := `
		SELECT number, status, accrual, uploaded_at
		FROM orders
		WHERE user_id = $1
		ORDER BY uploaded_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user orders: %w", err)
	}
	defer rows.Close()

	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.Number, &o.Status, &o.Accrual, &o.UploadedAt); err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return orders, nil
}

func (r *OrderRepository) Withdraw(ctx context.Context, order *Order) error {
	tx, err := r.db.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO orders (user_id, number, status, accrual)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (number) DO UPDATE
		SET status = EXCLUDED.status,
		    accrual = EXCLUDED.accrual
	`, order.UserID, order.Number, order.Status, order.Accrual)
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}

	return tx.Commit(ctx)
}

func (r *OrderRepository) CalculateUserBalance(ctx context.Context, userID string) (float64, error) {
	var balance *float64
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(accrual), 0)
		FROM orders
		WHERE user_id = $1
	`, userID).Scan(&balance)

	if err != nil {
		return 0, fmt.Errorf("calculate balance: %v", err)
	}

	return *balance, nil
}

func (r *OrderRepository) GetWithdrawals(ctx context.Context, userID string) ([]Order, error) {
	query := `
		SELECT number, accrual, uploaded_at
		FROM orders
		WHERE user_id = $1 AND status = 'WITHDRAWN'
		ORDER BY uploaded_at DESC
	`

	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.Number, &o.Accrual, &o.UploadedAt); err != nil {
			return nil, fmt.Errorf("failed to scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, o)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return withdrawals, nil
}

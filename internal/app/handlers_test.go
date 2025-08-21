package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/JohnnyConstantin/go_mart/internal/repository"
	"github.com/JohnnyConstantin/go_mart/internal/store"
	"github.com/JohnnyConstantin/go_mart/models"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Пришлось мокать бд, чтобы провести хотя бы валидационные тесты. Coverage в районе 30-40 %

// MockDatabase реализация для тестов
type MockDatabase struct {
	users          map[string]*repository.User
	orders         map[string]*repository.Order
	loginExists    bool
	orderExists    bool
	otherUserOrder bool
	shouldFail     bool
}

// MockTransaction реализация интерфейса store.Transaction для тестов
type MockTransaction struct {
	db *MockDatabase
}

func (mt *MockTransaction) BeginTx(ctx context.Context) (store.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func (mt *MockTransaction) Close() {
	//TODO implement me
	panic("implement me")
}

func (mt *MockTransaction) Commit(ctx context.Context) error {
	return nil
}

func (mt *MockTransaction) Rollback(ctx context.Context) error {
	return nil
}

func (mt *MockTransaction) Exec(ctx context.Context, query string, args ...interface{}) (int64, error) {
	return mt.db.Exec(ctx, query, args...)
}

func (mt *MockTransaction) QueryRow(ctx context.Context, query string, args ...interface{}) store.Row {
	return mt.db.QueryRow(ctx, query, args...)
}

func (mt *MockTransaction) Query(ctx context.Context, query string, args ...interface{}) (store.Rows, error) {
	return mt.db.Query(ctx, query, args...)
}

// MockRow реализация интерфейса store.Row для тестов
type MockRow struct {
	values  []interface{}
	current int
}

func (mr *MockRow) Scan(dest ...interface{}) error {
	if mr.current >= len(mr.values) {
		return errors.New("no more rows")
	}

	for i, d := range dest {
		if i >= len(mr.values) {
			break
		}

		// Типа реализация сканирования, взял из инета, просто чтобы замокать скан
		switch v := mr.values[mr.current].(type) {
		case int:
			if ptr, ok := d.(*int); ok {
				*ptr = v
			}
		case string:
			if ptr, ok := d.(*string); ok {
				*ptr = v
			}
		case float64:
			if ptr, ok := d.(*float64); ok {
				*ptr = v
			}
		case time.Time:
			if ptr, ok := d.(*time.Time); ok {
				*ptr = v
			}
		}
		mr.current++
	}
	return nil
}

// MockRows реализация интерфейса store.Rows для тестов
type MockRows struct {
	rows    [][]interface{}
	current int
}

func (mrs *MockRows) Close() {}

func (mrs *MockRows) Next() bool {
	return mrs.current < len(mrs.rows)
}

func (mrs *MockRows) Scan(dest ...interface{}) error {
	if mrs.current >= len(mrs.rows) {
		return errors.New("no more rows")
	}

	for i, d := range dest {
		if i >= len(mrs.rows[mrs.current]) {
			break
		}

		// Типа реализация сканирования, взял из инета, просто чтобы замокать скан
		switch v := mrs.rows[mrs.current][i].(type) {
		case int:
			if ptr, ok := d.(*int); ok {
				*ptr = v
			}
		case string:
			if ptr, ok := d.(*string); ok {
				*ptr = v
			}
		case float64:
			if ptr, ok := d.(*float64); ok {
				*ptr = v
			}
		case time.Time:
			if ptr, ok := d.(*time.Time); ok {
				*ptr = v
			}
		}
	}
	mrs.current++
	return nil
}

func (mrs *MockRows) Err() error {
	return nil
}

func (m *MockDatabase) BeginTx(ctx context.Context) (store.Transaction, error) {
	if m.shouldFail {
		return nil, errors.New("transaction begin error")
	}
	return &MockTransaction{db: m}, nil
}

func (m *MockDatabase) Exec(ctx context.Context, query string, args ...interface{}) (int64, error) {
	if m.shouldFail {
		return 0, errors.New("exec error")
	}
	// возвращаем 1 затронутую строку
	return 1, nil
}

func (m *MockDatabase) QueryRow(ctx context.Context, query string, args ...interface{}) store.Row {
	if m.shouldFail {
		return &MockRow{}
	}

	// возвращаем пустую строку
	return &MockRow{}
}

func (m *MockDatabase) Query(ctx context.Context, query string, args ...interface{}) (store.Rows, error) {
	if m.shouldFail {
		return nil, errors.New("query error")
	}

	// Простейшая реализация - возвращаем пустой набор строк
	return &MockRows{}, nil
}

func (m *MockDatabase) Close() {
	// Ничего не делаем
}

// Вынес в отдельную функцию работу с контекстом
func createTestContext(db store.Database, userID string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, dbKey, db)
	ctx = context.WithValue(ctx, loggerKey, *zap.NewExample().Sugar())
	if userID != "" {
		ctx = context.WithValue(ctx, userKey, userID)
	}
	return ctx
}

func TestRegister(t *testing.T) {
	tests := []struct {
		name         string
		request      models.RegistrationReq
		db           *MockDatabase
		expectedCode int
	}{
		{
			name: "short password",
			request: models.RegistrationReq{
				Login:    "testuser",
				Password: "short",
			},
			db:           &MockDatabase{users: make(map[string]*repository.User)},
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "empty fields",
			request: models.RegistrationReq{
				Login:    "",
				Password: "",
			},
			db:           &MockDatabase{users: make(map[string]*repository.User)},
			expectedCode: store.ErrNotValidFormatCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/register", bytes.NewReader(body))
			w := httptest.NewRecorder()
			req = req.WithContext(createTestContext(tt.db, ""))

			Register(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, resp.StatusCode)
			}

			// Check cookie for successful registration
			if tt.expectedCode == http.StatusOK {
				cookies := resp.Cookies()
				if len(cookies) == 0 {
					t.Error("expected auth cookie, got none")
				}
			}

			defer resp.Body.Close()
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name         string
		request      models.LoginReq
		db           *MockDatabase
		expectedCode int
	}{
		{
			name: "empty fields",
			request: models.LoginReq{
				Login:    "",
				Password: "",
			},
			db:           &MockDatabase{users: make(map[string]*repository.User)},
			expectedCode: store.ErrNotValidFormatCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup password check for mock user
			if user, exists := tt.db.users[tt.request.Login]; exists {
				user.Password = "$2a$10$N9qo8uLOickgx2ZMRZoMy.MQRqQz6W6NAwCcDM5RTWf6QJKZ2jJ0W" // хардкод хеша
				user.CheckPassword("password123")
			}

			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/login", bytes.NewReader(body))
			w := httptest.NewRecorder()
			req = req.WithContext(createTestContext(tt.db, ""))

			Login(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, resp.StatusCode)
			}

			// Check cookie for successful login
			if tt.expectedCode == http.StatusOK {
				cookies := resp.Cookies()
				if len(cookies) == 0 {
					t.Error("expected auth cookie, got none")
				}
			}

			defer resp.Body.Close()
		})
	}
}

func TestOrdersPOST(t *testing.T) {
	tests := []struct {
		name         string
		orderNumber  string
		userID       string
		db           *MockDatabase
		contentType  string
		expectedCode int
	}{
		{
			name:         "invalid content type",
			orderNumber:  "12345678903",
			userID:       "user1",
			db:           &MockDatabase{},
			contentType:  "application/json",
			expectedCode: store.ErrNotValidFormatCode,
		},
		{
			name:         "invalid order number format",
			orderNumber:  "123",
			userID:       "user1",
			db:           &MockDatabase{},
			contentType:  "text/plain",
			expectedCode: store.ErrOrderInvalidFormatCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/orders", bytes.NewReader([]byte(tt.orderNumber)))
			req.Header.Set("Content-Type", tt.contentType)
			w := httptest.NewRecorder()
			req = req.WithContext(createTestContext(tt.db, tt.userID))

			OrdersPOST(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, resp.StatusCode)
			}

			defer resp.Body.Close()
		})
	}
}

func TestOrdersGET(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		db           *MockDatabase
		expectedCode int
	}{
		{
			name:   "no orders",
			userID: "user1",
			db: &MockDatabase{
				orders: make(map[string]*repository.Order),
			},
			expectedCode: store.ErrNoContentCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/orders", nil)
			w := httptest.NewRecorder()
			req = req.WithContext(createTestContext(tt.db, tt.userID))

			OrdersGET(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, resp.StatusCode)
			}

			if tt.expectedCode == http.StatusOK {
				var orders []models.OrderResponse
				if err := json.NewDecoder(resp.Body).Decode(&orders); err != nil {
					t.Errorf("failed to decode response: %v", err)
				}
				if len(orders) == 0 {
					t.Error("expected orders, got none")
				}
			}

			defer resp.Body.Close()
		})
	}
}

func TestBalanceWithdraw(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		request      models.WithdrawRequest
		db           *MockDatabase
		expectedCode int
	}{
		{
			name:   "invalid order number",
			userID: "user1",
			request: models.WithdrawRequest{
				Order: "123",
				Sum:   50,
			},
			db:           &MockDatabase{},
			expectedCode: store.ErrOrderInvalidFormatCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.request)
			req := httptest.NewRequest("POST", "/balance/withdraw", bytes.NewReader(body))
			w := httptest.NewRecorder()
			req = req.WithContext(createTestContext(tt.db, tt.userID))

			BalanceWithdraw(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, resp.StatusCode)
			}

			defer resp.Body.Close()
		})
	}
}

func TestWithdrawals(t *testing.T) {
	tests := []struct {
		name         string
		userID       string
		db           *MockDatabase
		expectedCode int
	}{
		{
			name:   "no withdrawals",
			userID: "user1",
			db: &MockDatabase{
				orders: make(map[string]*repository.Order),
			},
			expectedCode: store.ErrNoContentCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/withdrawals", nil)
			w := httptest.NewRecorder()
			req = req.WithContext(createTestContext(tt.db, tt.userID))

			Withdrawals(w, req)

			resp := w.Result()
			if resp.StatusCode != tt.expectedCode {
				t.Errorf("expected status %d, got %d", tt.expectedCode, resp.StatusCode)
			}

			defer resp.Body.Close()

			if tt.expectedCode == http.StatusOK {
				var withdrawals []models.WithdrawalResponse
				if err := json.NewDecoder(resp.Body).Decode(&withdrawals); err != nil {
					t.Errorf("failed to decode response: %v", err)
				}
				if len(withdrawals) == 0 {
					t.Error("expected withdrawals, got none")
				}
			}

		})
	}
}

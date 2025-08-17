package repository

import (
	"context"
	"errors"
	"github.com/JohnnyConstantin/go_mart/internal/store"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
	"time"
)

type UserRepository struct {
	db store.Database
}

type User struct {
	ID        string    `json:"id"`
	Login     string    `json:"login"`
	Password  string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

// HashPassword Храним пароль безопасно (относительно)
func (u *User) HashPassword() error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashed)
	return nil
}

// CheckPassword проверяет соответствие пароля хешу
func (u *User) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) == nil
}

func NewUserRepository(db store.Database) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(user *User) error {

	ctx := context.Background()

	query := `
		INSERT INTO users (login, password)
		VALUES ($1, $2)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(ctx, query, user.Login, user.Password).
		Scan(&user.ID, &user.CreatedAt)

	if store.IsDuplicateKeyError(err) {
		return store.ErrLoginDuplicate
	}

	return err
}

func (r *UserRepository) GetByLogin(login string) (*User, error) {

	ctx := context.Background()

	query := `
		SELECT id, login, password, created_at
		FROM users
		WHERE login = $1
	`

	var user User
	err := r.db.QueryRow(ctx, query, login).
		Scan(&user.ID, &user.Login, &user.Password, &user.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("no such user")
	}

	return &user, err
}

func (r *UserRepository) Exists(ctx context.Context, login string) (bool, error) {
	query := `
		SELECT EXISTS(SELECT 1 FROM users WHERE login = $1)
	`

	var exists bool
	err := r.db.QueryRow(ctx, query, login).Scan(&exists)
	return exists, err
}

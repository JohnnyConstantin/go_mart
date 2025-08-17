package app

import (
	"encoding/json"
	"errors"
	"github.com/JohnnyConstantin/go_mart/internal/auth"
	"github.com/JohnnyConstantin/go_mart/internal/config"
	"github.com/JohnnyConstantin/go_mart/internal/repository"
	"github.com/JohnnyConstantin/go_mart/internal/store"
	"github.com/JohnnyConstantin/go_mart/models"
	"go.uber.org/zap"
	"net/http"
	"time"
)

// Register Регистрирует пользователя в БД
func Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Извлекаем бд из контекста
	db, ok := ctx.Value(dbKey).(store.Database)
	if !ok {
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Извлекаем логгер из контекста
	sugar, ok := ctx.Value(loggerKey).(zap.SugaredLogger)
	if !ok {
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Создаем слой репозитория для работы с пользователями
	userRepo := repository.NewUserRepository(db)

	// Парсим входные данные в модельку регистрации
	req := models.RegistrationReq

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sugar.Errorf("Register: failed to decode request: %v", err)
		http.Error(w, store.ErrNotValidFormat.Error(), store.ErrNotValidFormatCode)
		return
	}

	// Валидация входных данных
	if req.Login == "" || req.Password == "" {
		http.Error(w, "Login and password are required", store.ErrNotValidFormatCode)
		return
	}

	// Хоть какая-то валидация длины пароля (возможно по тестам не пройдет?)
	if len(req.Password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Создаем пользователя
	user := repository.User{
		Login:    req.Login,
		Password: req.Password,
	}

	// Хешируем пароль у объекта
	err := user.HashPassword()
	if err != nil {
		sugar.Errorf("Register: failed to hash password: %v", err)
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Сохраняем в БД
	if err := userRepo.Create(&user); err != nil {
		if errors.Is(err, store.ErrLoginDuplicate) {
			http.Error(w, store.ErrLoginDuplicate.Error(), store.ErrLoginDuplicateCode)
			return
		}
		sugar.Errorf("Register: failed to create user: %v", err)
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Переходим на следующий уровень логики и передаем секрет
	jwtService := auth.NewJWTService(config.Config.JWTSecret)

	// Генерируем JWT токен
	token, err := jwtService.GenerateToken(user.ID)
	if err != nil {
		sugar.Errorf("Register: failed to generate token: %v", err)
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Устанавливаем cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	// Возвращаем успешный ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("User is successfully registered."))
}

// Login Аутентификация пользователя
func Login(w http.ResponseWriter, r *http.Request) {

	req := models.LoginReq
	ctx := r.Context()

	// Извлекаем бд из контекста
	db, ok := ctx.Value(dbKey).(store.Database)
	if !ok {
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Извлекаем логгер из контекста
	sugar, ok := ctx.Value(loggerKey).(zap.SugaredLogger)
	if !ok {
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Создаем слой репозитория для работы с пользователями
	userRepo := repository.NewUserRepository(db)

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sugar.Errorf("Login: decode error: %v", err)
		http.Error(w, store.ErrNotValidFormat.Error(), store.ErrNotValidFormatCode)
		return
	}

	// Валидация
	if req.Login == "" || req.Password == "" {
		http.Error(w, store.ErrNotValidFormat.Error(), store.ErrNotValidFormatCode)
		return
	}

	// Получаем пользователя из БД
	user, err := userRepo.GetByLogin(req.Login)
	if err != nil {
		if errors.Is(err, store.ErrInvalidLoginPassword) {
			http.Error(w, store.ErrInvalidLoginPassword.Error(), store.ErrInvalidLoginPasswordCode)
			return
		}
		sugar.Errorf("Login: DB error: %v", err)
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Переходим на следующий уровень логики и передаем секрет
	jwtService := auth.NewJWTService(config.Config.JWTSecret)

	// Проверяем пароль
	if !user.CheckPassword(req.Password) {
		http.Error(w, store.ErrInvalidLoginPassword.Error(), store.ErrInvalidLoginPasswordCode)
		return
	}

	// Генерируем JWT токен
	token, err := jwtService.GenerateToken(user.ID)
	if err != nil {
		sugar.Errorf("Login: token generation error: %v", err)
		http.Error(w, `{"message":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Устанавливаем cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	// Возвращаем успешный ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

// OrdersPOST Добавление заказов
func OrdersPOST(w http.ResponseWriter, r *http.Request) {

	w.Write([]byte("I am orders post handler"))
	w.WriteHeader(http.StatusOK)
}

// OrdersGET Получение заказов
func OrdersGET(w http.ResponseWriter, r *http.Request) {

	w.Write([]byte("I am orders get handler"))
	w.WriteHeader(http.StatusOK)
}

// Balance Получение баланса кошелька
func Balance(w http.ResponseWriter, r *http.Request) {

	w.Write([]byte("I am balance handler"))
	w.WriteHeader(http.StatusOK)
}

// BalanceWithdraw Снятие денег с кошелька
func BalanceWithdraw(w http.ResponseWriter, r *http.Request) {

	w.Write([]byte("I am balance withdraw handler"))
	w.WriteHeader(http.StatusOK)
}

// Withdrawals Получение истории снятий с кошелька
func Withdrawals(w http.ResponseWriter, r *http.Request) {

	w.Write([]byte("I am withdraws handler"))
	w.WriteHeader(http.StatusOK)
}

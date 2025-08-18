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
	"sort"
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
}

// OrdersPOST Добавление заказов
func OrdersPOST(w http.ResponseWriter, r *http.Request) {

	if r.Header.Get("Content-Type") != "text/plain" {
		http.Error(w, store.ErrNotValidFormat.Error(), store.ErrNotValidFormatCode)
		return
	}

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

	// Извлекаем UserID из контекста
	userID, ok := ctx.Value(userKey).(string)
	if !ok {
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Читаем номер заказа
	number, err := repository.ReadOrderNumber(r)
	if err != nil {
		sugar.Errorf("OrdersPOST: failed to read order number: %v", err)
		http.Error(w, store.ErrNotValidFormat.Error(), store.ErrNotValidFormatCode)
		return
	}

	// Проверяем номер алгоритмом Луна
	if !repository.ValidateOrderNumber(number) {
		http.Error(w, store.ErrOrderInvalidFormat.Error(), store.ErrOrderInvalidFormatCode)
		return
	}

	// Создаем слой репозитория для работы с заказами
	orderRepo := repository.NewOrderRepository(db)

	// Создаем заказ
	order := repository.Order{
		UserID: userID,
		Number: number,
		Status: "NEW",
	}

	err = orderRepo.CreateOrder(ctx, &order)
	switch {
	case errors.Is(err, store.ErrOrderForOtherUser):
		http.Error(w, store.ErrOrderForOtherUser.Error(), store.ErrOrderForOtherUserCode)
		return
	case errors.Is(err, store.ErrOrderForThisUser):
		w.WriteHeader(store.ErrOrderForThisUserCode)
		return
	case err != nil:
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	default:
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("Order is successfully registered"))
		return
	}
}

// OrdersGET Получение заказов
func OrdersGET(w http.ResponseWriter, r *http.Request) {
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

	// Извлекаем UserID из контекста
	userID, ok := ctx.Value(userKey).(string)
	if !ok {
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Создаем слой репозитория для работы с заказами
	orderRepo := repository.NewOrderRepository(db)

	// Получаем заказы пользователя
	orders, err := orderRepo.GetUserOrders(ctx, userID)
	if err != nil {
		sugar.Errorf("OrdersGET: failed to get orders: %v", err)
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Если заказов нет - возвращаем 204
	if len(orders) == 0 {
		w.WriteHeader(store.ErrNoContentCode)
		w.Write([]byte(store.ErrNoContent.Error()))
		return
	}

	// Формируем ответ
	response := make([]models.OrderResponse, 0, len(orders))
	for _, order := range orders {
		resp := models.OrderResponse{
			Number:     order.Number,
			Status:     order.Status,
			UploadedAt: order.UploadedAt,
		}

		// Добавляем accrual только для PROCESSED статуса
		if order.Status == "PROCESSED" {
			resp.Accrual = &order.Accrual
		}

		response = append(response, resp)
	}

	// Сортируем по дате загрузки (новые сначала)
	sort.Slice(response, func(i, j int) bool {
		return response[i].UploadedAt.After(response[j].UploadedAt)
	})

	// Отправляем ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		sugar.Errorf("OrdersGET: failed to encode response: %v", err)
	}
}

// Balance Получение баланса кошелька
func Balance(w http.ResponseWriter, r *http.Request) {

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

	// Извлекаем UserID из контекста
	userID, ok := ctx.Value(userKey).(string)
	if !ok {
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Создаем слой репозитория для работы с заказами
	orderRepo := repository.NewOrderRepository(db)

	balance, err := orderRepo.CalculateUserBalance(ctx, userID)
	if err != nil {
		sugar.Errorf("BalanceWithdraw: get balance error: %v", err)
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Получаем списания (заказы со статусом WITHDRAWN)
	withdrawals, err := orderRepo.GetWithdrawals(ctx, userID)
	if err != nil {
		sugar.Errorf("WithdrawalsGET: failed to get withdrawals: %v", err)
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	withdrawn := 0.0

	for _, wd := range withdrawals {
		withdrawn += -1 * wd.Accrual
	}

	response := models.BalanceResponse{
		Current:   balance,
		Withdrawn: withdrawn,
	}

	// Отправляем ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		sugar.Errorf("OrdersGET: failed to encode response: %v", err)
	}

}

// BalanceWithdraw Снятие денег с кошелька
func BalanceWithdraw(w http.ResponseWriter, r *http.Request) {

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

	// Извлекаем UserID из контекста
	userID, ok := ctx.Value(userKey).(string)
	if !ok {
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	var req models.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sugar.Errorf("BalanceWithdraw: decode error: %v", err)
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Проверка номера заказа алгоритмом Луна
	if !repository.ValidateOrderNumber(req.Order) {
		http.Error(w, store.ErrOrderInvalidFormat.Error(), store.ErrOrderInvalidFormatCode)
		return
	}

	// Создаем слой репозитория для работы с заказами
	orderRepo := repository.NewOrderRepository(db)

	// Проверка баланса
	balance, err := orderRepo.CalculateUserBalance(ctx, userID)
	if err != nil {
		sugar.Errorf("BalanceWithdraw: get balance error: %v", err)
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	if balance < req.Sum {
		http.Error(w, store.ErrInsufficientBalance.Error(), store.ErrInsufficientBalanceCode)
		return
	}

	// Создаем запись о списании
	order := &repository.Order{
		UserID:  userID,
		Number:  req.Order,
		Status:  "WITHDRAWN",
		Accrual: -req.Sum, // Отрицательное значение
	}

	// Выполняем в транзакции
	err = orderRepo.Withdraw(ctx, order)
	if err != nil {
		sugar.Errorf("BalanceWithdraw: withdraw error: %v", err)
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Withdrawals Получение истории снятий с кошелька
func Withdrawals(w http.ResponseWriter, r *http.Request) {

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

	// Извлекаем UserID из контекста
	userID, ok := ctx.Value(userKey).(string)
	if !ok {
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Создаем слой репозитория для работы с заказами
	orderRepo := repository.NewOrderRepository(db)

	// Получаем списания (заказы со статусом WITHDRAWN)
	withdrawals, err := orderRepo.GetWithdrawals(ctx, userID)
	if err != nil {
		sugar.Errorf("WithdrawalsGET: failed to get withdrawals: %v", err)
		http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
		return
	}

	// Если списаний нет - возвращаем 204
	if len(withdrawals) == 0 {
		w.WriteHeader(store.ErrNoContentCode)
		return
	}

	// Формируем ответ (сумма без минуса)
	response := make([]models.WithdrawalResponse, 0, len(withdrawals))
	for _, wd := range withdrawals {
		response = append(response, models.WithdrawalResponse{
			Order:       wd.Number,
			Sum:         -wd.Accrual, // Убираем минус
			ProcessedAt: wd.UploadedAt,
		})
	}

	// Сортируем по дате (новые сначала)
	sort.Slice(response, func(i, j int) bool {
		return response[i].ProcessedAt.After(response[j].ProcessedAt)
	})

	// Отправляем ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		sugar.Errorf("WithdrawalsGET: failed to encode response: %v", err)
	}
}

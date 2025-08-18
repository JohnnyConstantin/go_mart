package models

import "time"

// RegistrationReq Запрос на регистрацию пользователя
type RegistrationReq struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// LoginReq Такой же, как RegistrationReq, но логически представляет из себя отдельный объект
type LoginReq struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// AccrualResult Получаемый возврат от внешнего сервиса
type AccrualResult struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}

// OrderResponse Возврат пользователю заказов
type OrderResponse struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    *float64  `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

// WithdrawRequest Запрос на списание средств
type WithdrawRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

// BalanceResponse Ответ на запрос баланса
type BalanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

// WithdrawalResponse Ответ на запрос всех списаний
type WithdrawalResponse struct {
	Order       string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

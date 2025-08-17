package models

import "time"

// RegistrationReq Запрос на регистрацию пользователя
var RegistrationReq struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// LoginReq Такой же, как RegistrationReq, но логически представляет из себя отдельный объект
var LoginReq struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// AccrualResult Получаемый возврат от внешнего сервиса
var AccrualResult struct {
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

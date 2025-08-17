package models

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

var AccrualResult struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}

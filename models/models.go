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

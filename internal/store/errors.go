package store

import "errors"

var (
	ErrNotValidFormat           = errors.New("invalid format of request")
	ErrNotValidFormatCode       = 400
	ErrLoginDuplicate           = errors.New("duplicate login")
	ErrLoginDuplicateCode       = 409
	ErrInternalServer           = errors.New("internal server error")
	ErrInternalServerCode       = 500
	ErrInvalidLoginPassword     = errors.New("invalid login/password")
	ErrInvalidLoginPasswordCode = 401
	ErrOrderForOtherUser        = errors.New("order exists for another user")
	ErrOrderForOtherUserCode    = 409
	ErrOrderInvalidFormat       = errors.New("invalid order format")
	ErrOrderInvalidFormatCode   = 422
	ErrOrderNotFound            = errors.New("order not found")
	ErrOrderForThisUser         = errors.New("order exists for this user")
	ErrOrderForThisUserCode     = 200
)

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
)

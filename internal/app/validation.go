package app

// ValidateOrderNumber Валидация номера ордера по алгоритму Луна
func ValidateOrderNumber(number string) bool {
	sum := 0
	alternate := false

	for i := len(number) - 1; i >= 0; i-- {
		digit := int(number[i] - '0')
		if alternate {
			digit *= 2
			if digit > 9 {
				digit = (digit % 10) + 1
			}
		}
		sum += digit
		alternate = !alternate
	}

	return sum%10 == 0
}

// LoginPassValidation Валидация пустых логина и пароля
func LoginPassValidation(login string, password string) bool {
	if login == "" || password == "" {
		return false
	}
	return true
}

func PasswordMinLengthValidation(password string) bool {
	if len(password) < 8 {
		return false
	}
	return true
}

package config

import (
	"flag"
)

var Config struct {
	ServerAddress  string
	DatabaseURL    string
	AccrualAddress string
	JWTSecret      string
}

func init() {
	flag.StringVar( // Адрес старта сервера
		&Config.ServerAddress,
		"a",
		"localhost:8080",
		"The address and port to start the server on",
	)
	flag.StringVar( // Адрес БД
		&Config.DatabaseURL,
		"d",
		"",
		"Database connection",
	)
	flag.StringVar( // Адрес системы рассчета начислений
		&Config.AccrualAddress,
		"r",
		"localhost:8080",
		"Address of a accrual subsystem",
	)
	flag.StringVar( // Адрес системы рассчета начислений
		&Config.JWTSecret,
		"j",
		"some_strong_secret",
		"Secret for jwt",
	)
}

package config

import (
	"flag"
	"os"
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

// GetConfig Перенес инициализацию параметров сюда. Флаги + енвы
func GetConfig() {
	flag.Parse()
	//Подгружаем переменные окружения при наличии
	envServerAddress, ok := os.LookupEnv("RUN_ADDRESS")
	if ok && envServerAddress != "" {
		Config.ServerAddress = envServerAddress
	}
	envDatabaseURL, ok := os.LookupEnv("DATABASE_URI")
	if ok && envDatabaseURL != "" {
		Config.DatabaseURL = envDatabaseURL
	}
	envAccrualAddress, ok := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS")
	if ok && envAccrualAddress != "" {
		Config.AccrualAddress = envAccrualAddress
	}
}

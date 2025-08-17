package main

import (
	"context"
	"flag"
	"github.com/JohnnyConstantin/go_mart/internal/accrual"
	"github.com/JohnnyConstantin/go_mart/internal/app"
	"github.com/JohnnyConstantin/go_mart/internal/config"
	"github.com/JohnnyConstantin/go_mart/internal/repository"
	"github.com/JohnnyConstantin/go_mart/internal/store"
	route "github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"log"
	"net/http"
	"os"
)

var sugar zap.SugaredLogger

func main() {

	//Создаём предустановленный регистратор zap
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	// Создали экземпляр и в дальнейшем прокидываем его в middleware с логированием
	sugar = *logger.Sugar()

	// Парсим флаги и енвы. Енвы вынесены в отдельную функцию
	flag.Parse()
	loadEnvs()

	// Инициализация подключения к БД
	db, err := initStorage()
	if err != nil {
		os.Exit(1)
	}

	//Если вернулся хендлер к БД (т.е. успешно создано соединение к БД), то закрываем после завершения программы
	if db != nil {
		defer db.Close()
	}

	//Используем внешний роутер chi
	router := route.NewRouter()

	// Инициализация хендлеров
	createHandlers(db, router, sugar)

	// записываем в лог, что сервер запускается
	sugar.Infow(
		"Starting server...",
		"addr", config.Config.ServerAddress,
	)

	orderRepo := repository.NewOrderRepository(db)

	// Инициализация процессора начислений
	accrualProcessor := accrual.NewProcessor(
		*orderRepo,
		config.Config.AccrualAddress,
		&sugar,
	)

	// Запуск в фоне
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	accrualProcessor.Start(ctx)

	err = http.ListenAndServe(config.Config.ServerAddress, router)
	if err != nil {
		return
	}
}

func createHandlers(db store.Database, router *route.Mux, sugar zap.SugaredLogger) {

	//Накидываем хендлеры на роуты
	router.Route("/", func(r route.Router) {
		r.Route("/api", func(r route.Router) {
			r.Route("/user", func(r route.Router) {
				r.Post("/register",
					app.GzipHandle( // Сжатие
						app.WithLogging(db, // Логирование, прокидываем в него регистратор логов sugar
							app.Register, sugar))) // Сам хендлер
				r.Post("/login",
					app.GzipHandle( // Сжатие
						app.WithLogging(db, // Логирование, прокидываем в него регистратор логов sugar
							app.Login, sugar))) // Сам хендлер
				r.Post("/orders",
					app.GzipHandle( // Сжатие
						app.WithLogging(db, // Логирование, прокидываем в него регистратор логов sugar
							app.WithAuth(
								app.OrdersPOST), sugar))) // Сам хендлер
				r.Get("/orders",
					app.GzipHandle( // Сжатие
						app.WithLogging(db, // Логирование, прокидываем в него регистратор логов sugar
							app.WithAuth(
								app.OrdersGET), sugar))) // Сам хендлер
				r.Get("/balance",
					app.GzipHandle( // Сжатие
						app.WithLogging(db, // Логирование, прокидываем в него регистратор логов sugar
							app.WithAuth(
								app.Balance), sugar))) // Сам хендлер
				r.Post("/balance/withdraw",
					app.GzipHandle( // Сжатие
						app.WithLogging(db, // Логирование, прокидываем в него регистратор логов sugar
							app.WithAuth(
								app.BalanceWithdraw), sugar))) // Сам хендлер
				r.Get("/withdrawals",
					app.GzipHandle( // Сжатие
						app.WithLogging(db, // Логирование, прокидываем в него регистратор логов sugar
							app.WithAuth(
								app.Withdrawals), sugar))) // Сам хендлер
			})
		})
	})
}

func loadEnvs() {
	//Подгружаем переменные окружения при наличии
	envServerAddress, ok := os.LookupEnv("RUN_ADDRESS")
	if ok && envServerAddress != "" {
		config.Config.ServerAddress = envServerAddress
	}
	envDatabaseURL, ok := os.LookupEnv("DATABASE_URI")
	if ok && envDatabaseURL != "" {
		config.Config.DatabaseURL = envDatabaseURL
	}
	envAccrualAddress, ok := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS")
	if ok && envAccrualAddress != "" {
		config.Config.AccrualAddress = envAccrualAddress
	}
}

func initStorage() (store.Database, error) {
	db, err := store.OpenDB(config.Config.DatabaseURL)
	if err != nil {
		sugar.Error("Could not connect to database")
		return nil, err
	}

	migrator := store.NewMigrator(db)
	if err := migrator.Migrate(context.Background()); err != nil {
		log.Fatalf("Failed to apply migrations: %v", err)
	}

	sugar.Infow("Using PostgreSQL",
		"DSN: ", config.Config.DatabaseURL)

	return db, nil
}

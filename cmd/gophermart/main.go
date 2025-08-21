package main

import (
	"context"
	"github.com/JohnnyConstantin/go_mart/internal/accrual"
	"github.com/JohnnyConstantin/go_mart/internal/app"
	"github.com/JohnnyConstantin/go_mart/internal/config"
	"github.com/JohnnyConstantin/go_mart/internal/repository"
	"github.com/JohnnyConstantin/go_mart/internal/store"
	route "github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"log"
	"net/http"
)

func main() {
	// Занес объявление переменной логгера внутрь main. Затем передаю в необходимые функции как объект
	var sugar zap.SugaredLogger

	//Создаём предустановленный регистратор zap
	logger, err := zap.NewDevelopment()
	if err != nil {
		// Фаталимся через вывод ошибки с последующим os.exit(1)
		log.Fatal("failed to initialize logger")
	}
	defer logger.Sync()

	// Создали экземпляр и в дальнейшем прокидываем его в middleware с логированием
	sugar = *logger.Sugar()

	// Парсим флаги и енвы
	config.GetConfig()

	// Инициализация подключения к БД
	db, err := initStorage(sugar)
	if err != nil {
		// Фаталимся через вывод ошибки с последующим os.exit(1)
		log.Fatal("Failed to initialize storage")
	}

	//Если вернулся хендлер к БД (т.е. успешно создано соединение к БД), то закрываем после завершения программы
	if db != nil {
		defer db.Close()
	}

	//Используем внешний роутер chi
	router := route.NewRouter()

	// Инициализация хендлеров
	app.CreateHandlers(db, router, sugar)

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

func initStorage(sugar zap.SugaredLogger) (store.Database, error) {
	ctx := context.Background()
	db, err := store.OpenDB(ctx, config.Config.DatabaseURL)
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

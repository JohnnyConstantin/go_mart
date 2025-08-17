package app

import (
	"compress/gzip"
	"context"
	"github.com/JohnnyConstantin/go_mart/internal/auth"
	"github.com/JohnnyConstantin/go_mart/internal/repository"
	"github.com/JohnnyConstantin/go_mart/internal/store"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

type (
	key       string
	logger    string
	myKeyType string
)

const (
	user      myKeyType = "user"
	loggerKey logger    = "sugar"
	dbKey     key       = "db"
)

func GzipHandle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Проверка на то, что клиент прислал пожатый контент
		if strings.Contains(r.Header.Get("Content-Encoding"), "gzip") {
			gz, err := gzip.NewReader(r.Body) // Распаковываем
			if err != nil {
				http.Error(w, "Invalid gzip body", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			r.Body = gz
		}

		originalWriter := w
		acceptsGzip := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") // Проверяем что клиент поддерживает сжатие

		if acceptsGzip { // Если клиент поддерживает сжатие, проверяем передаваемый content-type
			contentType := r.Header.Get("Content-Type")
			if strings.HasPrefix(contentType, "application/json") ||
				strings.HasPrefix(contentType, "text/html") {

				gzWriter := gzip.NewWriter(w) // Жмем!
				defer gzWriter.Close()

				w.Header().Set("Content-Encoding", "gzip") // Ставим заголовок, что пожали контент
				originalWriter = &gzipWriter{
					ResponseWriter: w,
					Writer:         gzWriter,
				}
			}
		}

		next(originalWriter, r) // перекидываем дальше
	}
}

func WithLogging(db store.Database, h http.HandlerFunc, logger zap.SugaredLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now() // Засекаем

		ctx := context.WithValue(r.Context(), dbKey, db)
		ctx = context.WithValue(ctx, loggerKey, logger)

		responseData := &responseData{
			status: 0,
			size:   0,
		}
		lw := loggingResponseWriter{
			ResponseWriter: w,
			responseData:   responseData,
		}
		// Прокидываем дальше
		h(&lw, r.WithContext(ctx))

		duration := time.Since(start) // Получаем время выполнения всех последующих middleware хендлеров

		logger.Infoln(
			"uri", r.RequestURI,
			"method", r.Method,
			"status", responseData.status,
			"duration", duration,
			"size", responseData.size,
		)
	}
}

func WithAuth(hf http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		ctx := r.Context()

		db, ok := ctx.Value(dbKey).(store.Database)
		if !ok {
			http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
			return
		}

		// Извлекаем логгер из контекста
		sugar, ok := ctx.Value(loggerKey).(zap.SugaredLogger)
		if !ok {
			http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
			return
		}

		cookie, err := r.Cookie("auth_token")
		if err != nil {
			sugar.Debug("AuthMiddleware: no auth cookie")
			http.Error(w, store.ErrInvalidLoginPassword.Error(), store.ErrInvalidLoginPasswordCode)
			return
		}

		// Валидация JWT и получение вшитого UserID
		userID, err := auth.ValidateJWTToken(cookie.Value)
		if err != nil {
			sugar.Debugf("AuthMiddleware: invalid token: %v", err)
			http.Error(w, store.ErrInvalidLoginPassword.Error(), store.ErrInvalidLoginPasswordCode)
			return
		}

		// Создаем слой репозитория для работы с пользователями
		userRepo := repository.NewUserRepository(db)

		// Проверка существования пользователя с таким ID
		exists, err := userRepo.Exists(r.Context(), userID)
		if err != nil {
			sugar.Errorf("AuthMiddleware: failed to check user: %v", err)
			http.Error(w, store.ErrInternalServer.Error(), store.ErrInternalServerCode)
			return
		}
		if !exists {
			sugar.Debugf("AuthMiddleware: user not found: %s", userID)
			http.Error(w, store.ErrInvalidLoginPassword.Error(), store.ErrInvalidLoginPasswordCode)
			return
		}

		// Добавляем userID в контекст
		ctx = context.WithValue(r.Context(), user, userID)
		ctx = context.WithValue(ctx, dbKey, db)
		ctx = context.WithValue(ctx, loggerKey, sugar)

		hf(w, r.WithContext(ctx))
	}
}

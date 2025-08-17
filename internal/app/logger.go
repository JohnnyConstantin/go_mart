package app

import (
	"context"
	"github.com/JohnnyConstantin/go_mart/internal/store"
	"go.uber.org/zap"
	"net/http"
	"time"
)

type key string
type logger string

const dbKey key = "db"
const loggerKey logger = "sugar"

type (
	// Берём структуру для хранения сведений об ответе
	responseData struct {
		status int
		size   int
	}

	// Добавляем реализацию http.ResponseWriter
	loggingResponseWriter struct {
		http.ResponseWriter // встраиваем оригинальный http.ResponseWriter
		responseData        *responseData
	}
)

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	// Записываем ответ, используя оригинальный http.ResponseWriter
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size // захватываем размер
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	// Записываем код статуса, используя оригинальный http.ResponseWriter
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode // захватываем код статуса
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

		logger.Infoln( // Логируем
			"uri", r.RequestURI,
			"method", r.Method,
			"status", responseData.status,
			"duration", duration,
			"size", responseData.size,
		)
	}
}

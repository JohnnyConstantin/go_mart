package auth

import (
	"github.com/JohnnyConstantin/go_mart/internal/config"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

// Тесты для проверки слоя JWT (в целом, конечно, бесполезные, но пусть будут)
func TestJWTService(t *testing.T) {
	secret := "test_secret"
	userID := "test_user_123"
	jwtService := NewJWTService(secret)

	t.Run("successful token generation and parsing", func(t *testing.T) {
		token, err := jwtService.GenerateToken(userID)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		parsedUserID, err := jwtService.ParseToken(token)
		assert.NoError(t, err)
		assert.Equal(t, userID, parsedUserID)
	})

	t.Run("invalid token parsing", func(t *testing.T) {
		_, err := jwtService.ParseToken("invalid.token.string")
		assert.Error(t, err)
	})

	t.Run("expired token", func(t *testing.T) {
		// Создаем токен с истекшим сроком действия
		expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
			UserID: userID,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			},
		})
		tokenString, _ := expiredToken.SignedString([]byte(secret))

		_, err := jwtService.ParseToken(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is expired")
	})

	t.Run("wrong secret key", func(t *testing.T) {
		token, err := jwtService.GenerateToken(userID)
		assert.NoError(t, err)

		// Создаем сервис с другим секретным ключом
		wrongService := NewJWTService("wrong_secret")
		_, err = wrongService.ParseToken(token)
		assert.Error(t, err)
	})
}

func TestValidateJWTToken(t *testing.T) {
	// Устанавливаем тестовый секрет
	config.Config.JWTSecret = "test_secret"
	userID := "test_user_123"

	t.Run("successful validation", func(t *testing.T) {
		// Используем сервис для генерации валидного токена
		jwtService := NewJWTService(config.Config.JWTSecret)
		token, err := jwtService.GenerateToken(userID)
		assert.NoError(t, err)

		validatedUserID, err := ValidateJWTToken(token)
		assert.NoError(t, err)
		assert.Equal(t, userID, validatedUserID)
	})

	t.Run("invalid token", func(t *testing.T) {
		_, err := ValidateJWTToken("invalid.token.string")
		assert.Error(t, err)
	})

	t.Run("malformed", func(t *testing.T) {
		// Создаем токен с неправильным методом подписи
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"user_id": userID,
		})
		tokenString, _ := token.SignedString([]byte(config.Config.JWTSecret))

		_, err := ValidateJWTToken(tokenString)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token is malformed")
	})
}

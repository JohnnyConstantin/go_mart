package auth

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"time"
)

type Service interface {
	GenerateToken(userID string) (string, error)
	ParseToken(token string) (string, error)
}

type JWTService struct {
	secret string
}

func NewJWTService(secret string) Service {
	return &JWTService{secret: secret}
}

type claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func (s *JWTService) GenerateToken(userID string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	})

	return token.SignedString([]byte(s.secret))
}

func (s *JWTService) ParseToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.secret), nil
	})
	if err != nil {
		return "", err
	}

	if claims, ok := token.Claims.(*claims); ok && token.Valid {
		return claims.UserID, nil
	}

	return "", errors.New("invalid token")
}

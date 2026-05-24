package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims — содержимое JWT. role: 0 — обычный пользователь, 1 — модератор.
type Claims struct {
	UserID uint `json:"user_id"`
	Role   int  `json:"role"`
	jwt.RegisteredClaims
}

// секрет берётся из JWT_SECRET; fallback для локальной разработки
func secret() []byte {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		s = "dev-secret-change-me"
	}
	return []byte(s)
}

// GenerateToken — выпускает JWT на 24 часа с уникальным jti.
func GenerateToken(userID uint, role int) (token string, jti string, expiresIn int64, err error) {
	jti = fmt.Sprintf("%d-%d", userID, time.Now().UnixNano())
	exp := time.Now().Add(24 * time.Hour)

	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(secret())
	if err != nil {
		return "", "", 0, err
	}
	return signed, jti, int64(time.Until(exp).Seconds()), nil
}

// ParseToken — разбирает JWT, возвращает claims или ошибку.
func ParseToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return secret(), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

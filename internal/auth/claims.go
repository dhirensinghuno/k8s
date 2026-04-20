package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
	jwt.RegisteredClaims
}

func NewClaims(userID int, username string, role Role, expiry time.Time) *Claims {
	return &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiry),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
}

func DefaultTokenExpiry() time.Time {
	return time.Now().Add(12 * time.Hour)
}

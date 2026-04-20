package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AuthMiddleware struct {
	jwtSecret string
	store     UserStore
	enabled   bool
}

type UserStore interface {
	GetUserByUsername(username string) (*User, error)
	CreateUser(user *User) error
	GetAllUsers() ([]User, error)
	DeleteUser(id int) error
	UpdateUserRole(id int, role Role) error
}

func NewAuthMiddleware(secret string, store UserStore, enabled bool) *AuthMiddleware {
	return &AuthMiddleware{
		jwtSecret: secret,
		store:     store,
		enabled:   enabled,
	}
}

func (am *AuthMiddleware) GenerateToken(user *User) (string, error) {
	claims := NewClaims(user.ID, user.Username, user.Role, DefaultTokenExpiry())
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(am.jwtSecret))
}

func (am *AuthMiddleware) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(am.jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}

func (am *AuthMiddleware) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return ""
	}
	return parts[1]
}

func (am *AuthMiddleware) Handler() func(http.Handler) http.Handler {
	log.Printf("[AuthMiddleware] Handler() called")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !am.enabled {
				next.ServeHTTP(w, r)
				return
			}

			if r.URL.Path == "/api/auth/login" || r.URL.Path == "/health" || r.URL.Path == "/api/health" {
				next.ServeHTTP(w, r)
				return
			}

			tokenString := am.extractToken(r)
			if tokenString == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			claims, err := am.ValidateToken(tokenString)
			if err != nil {
				log.Printf("[Auth] Invalid token: %v", err)
				http.Error(w, fmt.Sprintf("Invalid token: %v", err), http.StatusUnauthorized)
				return
			}

			r.Header.Set("X-User-ID", fmt.Sprint(claims.UserID))
			r.Header.Set("X-Username", claims.Username)
			r.Header.Set("X-User-Role", string(claims.Role))

			next.ServeHTTP(w, r)
		})
	}
}

func (am *AuthMiddleware) RequireRole(allowedRoles ...Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !am.enabled {
				next.ServeHTTP(w, r)
				return
			}

			roleStr := r.Header.Get("X-User-Role")
			userRole := Role(roleStr)

			for _, allowed := range allowedRoles {
				if userRole == allowed {
					next.ServeHTTP(w, r)
					return
				}
			}

			log.Printf("[Auth] Forbidden: user %s with role %s tried to access %s",
				r.Header.Get("X-Username"), userRole, r.URL.Path)
			http.Error(w, "Insufficient permissions", http.StatusForbidden)
		})
	}
}

func (am *AuthMiddleware) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password required", http.StatusBadRequest)
		return
	}

	user, err := am.store.GetUserByUsername(req.Username)
	if err != nil {
		log.Printf("[Auth] Login failed: user not found: %s", req.Username)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !user.CheckPassword(req.Password) {
		log.Printf("[Auth] Login failed: invalid password for user: %s", req.Username)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := am.GenerateToken(user)
	if err != nil {
		log.Printf("[Auth] Login failed: token generation error: %v", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	log.Printf("[Auth] Login successful: user=%s role=%s", user.Username, user.Role)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":   token,
		"role":    string(user.Role),
		"user":    user.Username,
		"expires": DefaultTokenExpiry().Format(time.RFC3339),
	})
}

func (am *AuthMiddleware) IsEnabled() bool {
	return am.enabled
}

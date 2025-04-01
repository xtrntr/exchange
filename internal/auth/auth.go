package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/xtrntr/exchange/internal/db"
	"github.com/xtrntr/exchange/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles user authentication
type AuthService struct {
	DB *db.DB
}

// NewAuthService creates a new auth service
func NewAuthService(db *db.DB) *AuthService {
	return &AuthService{DB: db}
}

// Register creates a new user with hashed password
func (s *AuthService) Register(ctx context.Context, username, password string) (*models.User, error) {
	// Validate input
	if username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}
	if password == "" {
		return nil, fmt.Errorf("password cannot be empty")
	}
	if len(username) > 50 {
		return nil, fmt.Errorf("username too long (max 50 characters)")
	}
	if len(password) > 100 {
		return nil, fmt.Errorf("password too long (max 100 characters)")
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// Create user in database
	user, err := s.DB.CreateUser(ctx, username, string(hashedPassword))
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return user, nil
}

// Login verifies credentials and generates a JWT
func (s *AuthService) Login(ctx context.Context, username, password string) (string, error) {
	// Get user from database
	user, err := s.DB.GetUserByUsername(ctx, username)
	if err != nil {
		return "", err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", err
	}

	// Generate JWT
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	})

	// Sign token with a secret key (in production, use env variable)
	tokenString, err := token.SignedString([]byte("my-secret-key"))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

// GetUserFromToken extracts user ID from JWT
func (s *AuthService) GetUserFromToken(tokenString string) (int, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte("my-secret-key"), nil
	})
	if err != nil {
		return 0, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID, ok := claims["user_id"].(float64)
		if !ok {
			return 0, err
		}
		return int(userID), nil
	}
	return 0, err
}

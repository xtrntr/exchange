package auth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xtrntr/exchange/internal/db"
	"golang.org/x/crypto/bcrypt"
)

var testDB *db.DB

func TestMain(m *testing.M) {
	pool, err := pgxpool.New(context.Background(), "postgres://exchange_user:exchange_pass@localhost:5432/exchange_db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Apply migration if not already applied
	migration, err := os.ReadFile("../../migrations/001_init.sql")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read migration: %v\n", err)
		os.Exit(1)
	}
	_, err = pool.Exec(context.Background(), string(migration))
	if err != nil && !strings.Contains(err.Error(), "already exists") {
		fmt.Fprintf(os.Stderr, "Unable to apply migration: %v\n", err)
		os.Exit(1)
	}

	testDB, err = db.NewDB(context.Background(), "postgres://exchange_user:exchange_pass@localhost:5432/exchange_db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create DB: %v\n", err)
		os.Exit(1)
	}

	// Truncate tables before running tests
	_, err = pool.Exec(context.Background(), "TRUNCATE TABLE users, orders, trades RESTART IDENTITY")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to truncate tables: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func TestAuthService_Register(t *testing.T) {
	s := &AuthService{DB: testDB}

	tests := []struct {
		name        string
		username    string
		password    string
		expectError bool
	}{
		{
			name:        "Success",
			username:    "alice",
			password:    "password123",
			expectError: false,
		},
		{
			name:        "EmptyUsername",
			username:    "",
			password:    "password123",
			expectError: true,
		},
		{
			name:        "EmptyPassword",
			username:    "bob",
			password:    "",
			expectError: true,
		},
		{
			name:        "DuplicateUsername",
			username:    "alice",
			password:    "newpass",
			expectError: true,
		},
		{
			name:        "LongUsername",
			username:    strings.Repeat("a", 1000),
			password:    "password123",
			expectError: true, // Should fail due to VARCHAR(50) limit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up before each test
			ctx := context.Background()
			_, err := testDB.Pool.Exec(ctx, "TRUNCATE TABLE users, orders, trades RESTART IDENTITY")
			if err != nil {
				t.Fatalf("Failed to clean up database: %v", err)
			}

			// For duplicate test, ensure the user exists first
			if tt.name == "DuplicateUsername" {
				_, err := s.Register(ctx, "alice", "password123")
				if err != nil {
					t.Fatalf("Failed to create user for duplicate test: %v", err)
				}
			}

			user, err := s.Register(ctx, tt.username, tt.password)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if user.Username != tt.username {
				t.Errorf("expected username %q, got %q", tt.username, user.Username)
			}
			// Verify in database
			var storedHash string
			err = testDB.Pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE username=$1", tt.username).Scan(&storedHash)
			if err != nil {
				t.Errorf("user not found in DB: %v", err)
			}
			if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(tt.password)); err != nil {
				t.Errorf("password hash mismatch")
			}
		})
	}
}

func TestAuthService_Login(t *testing.T) {
	s := &AuthService{DB: testDB}
	s.Register(context.Background(), "alice", "password123")

	tests := []struct {
		name        string
		username    string
		password    string
		expectError bool
	}{
		{
			name:        "Success",
			username:    "alice",
			password:    "password123",
			expectError: false,
		},
		{
			name:        "WrongPassword",
			username:    "alice",
			password:    "wrongpass",
			expectError: true,
		},
		{
			name:        "NonExistentUser",
			username:    "bob",
			password:    "password123",
			expectError: true,
		},
		{
			name:        "LongPassword",
			username:    "alice",
			password:    strings.Repeat("p", 1000),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := s.Login(context.Background(), tt.username, tt.password)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			// Verify token
			parsed, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
				return []byte("my-secret-key"), nil
			})
			if err != nil {
				t.Errorf("invalid token: %v", err)
			}
			claims, ok := parsed.Claims.(jwt.MapClaims)
			if !ok || claims["username"] != "alice" {
				t.Errorf("invalid token claims")
			}
		})
	}
}

func TestAuthService_GetUserFromToken(t *testing.T) {
	s := &AuthService{DB: testDB}
	s.Register(context.Background(), "alice", "password123")
	token, _ := s.Login(context.Background(), "alice", "password123")

	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  float64(1),
		"username": "alice",
		"exp":      time.Now().Add(-time.Hour).Unix(),
	})
	expiredTokenStr, _ := expiredToken.SignedString([]byte("my-secret-key"))
	invalidToken, _ := expiredToken.SignedString([]byte("wrong-key"))

	tests := []struct {
		name         string
		token        string
		expectUserID int
		expectError  bool
	}{
		{
			name:         "Success",
			token:        token,
			expectUserID: 1,
			expectError:  false,
		},
		{
			name:        "ExpiredToken",
			token:       expiredTokenStr,
			expectError: true,
		},
		{
			name:        "InvalidSignature",
			token:       invalidToken,
			expectError: true,
		},
		{
			name:        "EmptyToken",
			token:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, err := s.GetUserFromToken(tt.token)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if userID != tt.expectUserID {
				t.Errorf("expected user ID %d, got %d", tt.expectUserID, userID)
			}
		})
	}
}

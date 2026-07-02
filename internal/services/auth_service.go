package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"

	"github.com/lib/pq"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/auth"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/models"
)

// isUniqueConstraintError returns true when err is a unique-constraint violation
// from either PostgreSQL (via lib/pq) or SQLite.
func isUniqueConstraintError(err error) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return true
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

var usernameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,32}$`)

type AuthService struct {
	db           *sql.DB
	tokenManager *auth.TokenManager
	logger       *slog.Logger
}

type RegisterInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func NewAuthService(db *sql.DB, tokenManager *auth.TokenManager, logger *slog.Logger) *AuthService {
	return &AuthService{db: db, tokenManager: tokenManager, logger: logger}
}

func (s *AuthService) Register(ctx context.Context, input RegisterInput) (models.User, auth.TokenPair, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.Email = strings.TrimSpace(strings.ToLower(input.Email))

	if !usernameRE.MatchString(input.Username) {
		return models.User{}, auth.TokenPair{}, errors.New("username must be 3-32 chars and use alphanumeric, underscore, or dash")
	}
	if _, err := mail.ParseAddress(input.Email); err != nil {
		return models.User{}, auth.TokenPair{}, errors.New("invalid email address")
	}
	if len(input.Password) < 8 {
		return models.User{}, auth.TokenPair{}, errors.New("password must be at least 8 characters")
	}

	passwordHash, err := auth.HashPassword(input.Password)
	if err != nil {
		return models.User{}, auth.TokenPair{}, fmt.Errorf("hash password: %w", err)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.User{}, auth.TokenPair{}, err
	}
	defer tx.Rollback()

	var user models.User
	err = tx.QueryRowContext(ctx, `
        INSERT INTO users (username, email, password_hash)
        VALUES ($1, $2, $3)
        RETURNING id, username, email, is_public, created_at
    `, input.Username, input.Email, passwordHash).Scan(&user.ID, &user.Username, &user.Email, &user.IsPublic, &user.CreatedAt)
	if err != nil {
		if isUniqueConstraintError(err) {
			return models.User{}, auth.TokenPair{}, errors.New("username or email already exists")
		}
		return models.User{}, auth.TokenPair{}, err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO collections (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, user.ID); err != nil {
		return models.User{}, auth.TokenPair{}, err
	}

	pair, err := s.tokenManager.GenerateTokenPair(user)
	if err != nil {
		return models.User{}, auth.TokenPair{}, err
	}

	if err := auth.StoreRefreshTokenTx(ctx, tx, user.ID, pair.RefreshToken, pair.RefreshExpiresAt); err != nil {
		return models.User{}, auth.TokenPair{}, err
	}

	if err := tx.Commit(); err != nil {
		return models.User{}, auth.TokenPair{}, err
	}
	return user, pair, nil
}

func (s *AuthService) Login(ctx context.Context, input LoginInput) (models.User, auth.TokenPair, error) {
	input.Username = strings.TrimSpace(input.Username)
	if input.Username == "" || input.Password == "" {
		return models.User{}, auth.TokenPair{}, errors.New("username and password are required")
	}

	var user models.User
	err := s.db.QueryRowContext(ctx, `
        SELECT id, username, email, password_hash, is_public, created_at
        FROM users WHERE username = $1
    `, input.Username).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.IsPublic, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, auth.TokenPair{}, errors.New("invalid credentials")
		}
		return models.User{}, auth.TokenPair{}, err
	}
	if err := auth.ComparePassword(user.PasswordHash, input.Password); err != nil {
		return models.User{}, auth.TokenPair{}, errors.New("invalid credentials")
	}

	pair, err := s.tokenManager.GenerateTokenPair(user)
	if err != nil {
		return models.User{}, auth.TokenPair{}, err
	}
	if err := auth.StoreRefreshToken(ctx, s.db, user.ID, pair.RefreshToken, pair.RefreshExpiresAt); err != nil {
		return models.User{}, auth.TokenPair{}, err
	}
	return user, pair, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (auth.TokenPair, error) {
	claims, err := s.tokenManager.ValidateToken(refreshToken, auth.TokenTypeRefresh)
	if err != nil {
		return auth.TokenPair{}, err
	}
	if err := auth.ValidateRefreshTokenStorage(ctx, s.db, refreshToken); err != nil {
		return auth.TokenPair{}, err
	}

	var user models.User
	err = s.db.QueryRowContext(ctx, `SELECT id, username, email, is_public, created_at FROM users WHERE id = $1`, claims.UserID).
		Scan(&user.ID, &user.Username, &user.Email, &user.IsPublic, &user.CreatedAt)
	if err != nil {
		return auth.TokenPair{}, err
	}

	if err := auth.RevokeRefreshToken(ctx, s.db, refreshToken); err != nil {
		return auth.TokenPair{}, err
	}

	pair, err := s.tokenManager.GenerateTokenPair(user)
	if err != nil {
		return auth.TokenPair{}, err
	}
	if err := auth.StoreRefreshToken(ctx, s.db, user.ID, pair.RefreshToken, pair.RefreshExpiresAt); err != nil {
		return auth.TokenPair{}, err
	}
	return pair, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return nil
	}
	return auth.RevokeRefreshToken(ctx, s.db, refreshToken)
}

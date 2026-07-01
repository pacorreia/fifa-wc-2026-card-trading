package auth_test

import (
	"testing"
	"time"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/auth"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/models"
)

func TestPasswordHashing(t *testing.T) {
	hash, err := auth.HashPassword("super-secret")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	if err := auth.ComparePassword(hash, "super-secret"); err != nil {
		t.Fatalf("compare password: %v", err)
	}
	if err := auth.ComparePassword(hash, "wrong"); err == nil {
		t.Fatalf("expected wrong password to fail")
	}
}

func TestJWTGenerationAndValidation(t *testing.T) {
	manager := auth.NewTokenManager("secret", 15*time.Minute, time.Hour)
	pair, err := manager.GenerateTokenPair(models.User{ID: 7, Username: "collector"})
	if err != nil {
		t.Fatalf("generate token pair: %v", err)
	}
	accessClaims, err := manager.ValidateToken(pair.AccessToken, auth.TokenTypeAccess)
	if err != nil {
		t.Fatalf("validate access token: %v", err)
	}
	if accessClaims.UserID != 7 || accessClaims.Username != "collector" {
		t.Fatalf("unexpected access claims: %+v", accessClaims)
	}
	if _, err := manager.ValidateToken(pair.RefreshToken, auth.TokenTypeRefresh); err != nil {
		t.Fatalf("validate refresh token: %v", err)
	}
}

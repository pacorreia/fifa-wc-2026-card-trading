package services

import (
	"testing"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/models"
)

func TestComputeTradeMatch(t *testing.T) {
	match := ComputeTradeMatch(
		"rival",
		2,
		[]models.Sticker{{Code: "ARGS1"}, {Code: "BRAS1"}},
		[]models.Sticker{{Code: "WCS001"}},
	)
	if match.UserID != 2 || match.Username != "rival" {
		t.Fatalf("unexpected identity: %+v", match)
	}
	if match.MatchScore != 3 {
		t.Fatalf("expected match score 3, got %d", match.MatchScore)
	}
}

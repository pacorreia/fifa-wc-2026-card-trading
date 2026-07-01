package services

import (
	"testing"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/models"
)

func TestCalculateCollectionStats(t *testing.T) {
	items := []models.CollectionItem{
		{HaveCount: 1, Sticker: models.Sticker{Code: "ARGS1", Team: "ARG"}},
		{HaveCount: 0, Sticker: models.Sticker{Code: "ARGP1", Team: "ARG"}},
		{HaveCount: 3, Sticker: models.Sticker{Code: "BRAS1", Team: "BRA", IsSpecial: true}},
		{HaveCount: 0, Sticker: models.Sticker{Code: "WCS001", Team: "WC2026", IsSpecial: true}},
	}
	stats := CalculateCollectionStats(items)
	if stats.Total != 4 || stats.Owned != 2 || stats.Missing != 2 {
		t.Fatalf("unexpected totals: %+v", stats)
	}
	if stats.Duplicates != 2 {
		t.Fatalf("expected 2 duplicates, got %d", stats.Duplicates)
	}
	if stats.SpecialsOwned != 1 || stats.SpecialsTotal != 2 {
		t.Fatalf("unexpected special stats: %+v", stats)
	}
}

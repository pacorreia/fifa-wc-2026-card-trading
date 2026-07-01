package services

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sort"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/models"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/ws"
)

// ErrPrivateCollection is returned when a requester tries to access another
// user's collection that has is_public set to false.
var ErrPrivateCollection = errors.New("collection is private")

type EventPublisher interface {
	PublishToUser(userID int64, event models.Event) error
	PublishGlobal(event models.Event) error
}

type CollectionService struct {
	db        *sql.DB
	logger    *slog.Logger
	publisher EventPublisher
}

func NewCollectionService(db *sql.DB, logger *slog.Logger, publisher EventPublisher) *CollectionService {
	return &CollectionService{db: db, logger: logger, publisher: publisher}
}

func (s *CollectionService) ensureCollection(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO collections (user_id) VALUES ($1) ON CONFLICT (user_id) DO NOTHING`, userID)
	return err
}

func (s *CollectionService) GetCollection(ctx context.Context, userID int64) ([]models.CollectionItem, error) {
	if err := s.ensureCollection(ctx, userID); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
        SELECT c.id,
               s.code,
               COALESCE(ci.have_count, 0),
               COALESCE(ci.updated_at, c.updated_at),
               s.code, s.team, s.category, s.is_special, s.position
        FROM collections c
        JOIN stickers s ON TRUE
        LEFT JOIN collection_items ci ON ci.collection_id = c.id AND ci.sticker_code = s.code
        WHERE c.user_id = $1
        ORDER BY s.team, s.code
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]models.CollectionItem, 0)
	for rows.Next() {
		var item models.CollectionItem
		if err := rows.Scan(
			&item.CollectionID,
			&item.StickerCode,
			&item.HaveCount,
			&item.UpdatedAt,
			&item.Sticker.Code,
			&item.Sticker.Team,
			&item.Sticker.Category,
			&item.Sticker.IsSpecial,
			&item.Sticker.Position,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *CollectionService) UpdateItem(ctx context.Context, userID int64, stickerCode string, haveCount int) (models.CollectionItem, error) {
	if haveCount < 0 || haveCount > 99 {
		return models.CollectionItem{}, errors.New("have_count must be between 0 and 99")
	}
	if err := s.ensureCollection(ctx, userID); err != nil {
		return models.CollectionItem{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.CollectionItem{}, err
	}
	defer tx.Rollback()

	var collectionID int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM collections WHERE user_id = $1`, userID).Scan(&collectionID); err != nil {
		return models.CollectionItem{}, err
	}

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM stickers WHERE code = $1)`, stickerCode).Scan(&exists); err != nil {
		return models.CollectionItem{}, err
	}
	if !exists {
		return models.CollectionItem{}, errors.New("sticker not found")
	}

	_, err = tx.ExecContext(ctx, `
        INSERT INTO collection_items (collection_id, sticker_code, have_count)
        VALUES ($1, $2, $3)
        ON CONFLICT (collection_id, sticker_code)
        DO UPDATE SET have_count = EXCLUDED.have_count, updated_at = NOW()
    `, collectionID, stickerCode, haveCount)
	if err != nil {
		return models.CollectionItem{}, err
	}

	var item models.CollectionItem
	err = tx.QueryRowContext(ctx, `
        SELECT ci.collection_id, ci.sticker_code, ci.have_count, ci.updated_at,
               s.code, s.team, s.category, s.is_special, s.position
        FROM collection_items ci
        JOIN stickers s ON s.code = ci.sticker_code
        WHERE ci.collection_id = $1 AND ci.sticker_code = $2
    `, collectionID, stickerCode).Scan(
		&item.CollectionID,
		&item.StickerCode,
		&item.HaveCount,
		&item.UpdatedAt,
		&item.Sticker.Code,
		&item.Sticker.Team,
		&item.Sticker.Category,
		&item.Sticker.IsSpecial,
		&item.Sticker.Position,
	)
	if err != nil {
		return models.CollectionItem{}, err
	}

	if err := tx.Commit(); err != nil {
		return models.CollectionItem{}, err
	}

	stats, err := s.GetStats(ctx, userID)
	if err == nil && s.publisher != nil {
		if evt, evtErr := ws.NewEvent(ws.EventCollectionUpdated, item); evtErr == nil {
			_ = s.publisher.PublishToUser(userID, evt)
		} else {
			s.logger.Error("create collection.updated event", "error", evtErr)
		}
		if evt, evtErr := ws.NewEvent(ws.EventStatsUpdated, stats); evtErr == nil {
			_ = s.publisher.PublishToUser(userID, evt)
		} else {
			s.logger.Error("create stats.updated event", "error", evtErr)
		}
	}
	return item, nil
}

func (s *CollectionService) GetStats(ctx context.Context, userID int64) (models.CollectionStats, error) {
	items, err := s.GetCollection(ctx, userID)
	if err != nil {
		return models.CollectionStats{}, err
	}
	return CalculateCollectionStats(items), nil
}

func CalculateCollectionStats(items []models.CollectionItem) models.CollectionStats {
	stats := models.CollectionStats{}
	teamMap := map[string]*models.TeamCompletion{}

	for _, item := range items {
		stats.Total++
		if item.HaveCount > 0 {
			stats.Owned++
		} else {
			stats.Missing++
		}
		if item.HaveCount > 1 {
			stats.Duplicates += item.HaveCount - 1
		}

		team := teamMap[item.Sticker.Team]
		if team == nil {
			team = &models.TeamCompletion{Team: item.Sticker.Team}
			teamMap[item.Sticker.Team] = team
		}
		team.Total++
		if item.HaveCount > 0 {
			team.Owned++
		}

		if item.Sticker.IsSpecial {
			stats.SpecialsTotal++
			if item.HaveCount > 0 {
				stats.SpecialsOwned++
			}
		}
	}

	if stats.Total > 0 {
		stats.OverallPercent = float64(stats.Owned) / float64(stats.Total) * 100
	}
	if stats.SpecialsTotal > 0 {
		stats.SpecialsPct = float64(stats.SpecialsOwned) / float64(stats.SpecialsTotal) * 100
	}

	stats.TeamStats = make([]models.TeamCompletion, 0, len(teamMap))
	for _, team := range teamMap {
		if team.Total > 0 {
			team.Percent = float64(team.Owned) / float64(team.Total) * 100
		}
		stats.TeamStats = append(stats.TeamStats, *team)
	}
	sort.Slice(stats.TeamStats, func(i, j int) bool { return stats.TeamStats[i].Team < stats.TeamStats[j].Team })
	return stats
}

func (s *CollectionService) GetMissing(ctx context.Context, userID int64) ([]models.Sticker, error) {
	if err := s.ensureCollection(ctx, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT s.code, s.team, s.category, s.is_special, s.position
        FROM collections c
        JOIN stickers s ON TRUE
        LEFT JOIN collection_items ci ON ci.collection_id = c.id AND ci.sticker_code = s.code
        WHERE c.user_id = $1 AND COALESCE(ci.have_count, 0) = 0
        ORDER BY s.team, s.code
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stickers []models.Sticker
	for rows.Next() {
		var sticker models.Sticker
		if err := rows.Scan(&sticker.Code, &sticker.Team, &sticker.Category, &sticker.IsSpecial, &sticker.Position); err != nil {
			return nil, err
		}
		stickers = append(stickers, sticker)
	}
	return stickers, rows.Err()
}

func (s *CollectionService) GetDuplicates(ctx context.Context, userID int64) ([]models.CollectionItem, error) {
	if err := s.ensureCollection(ctx, userID); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
        SELECT ci.collection_id, ci.sticker_code, ci.have_count, ci.updated_at,
               s.code, s.team, s.category, s.is_special, s.position
        FROM collections c
        JOIN collection_items ci ON ci.collection_id = c.id
        JOIN stickers s ON s.code = ci.sticker_code
        WHERE c.user_id = $1 AND ci.have_count > 1
        ORDER BY s.team, s.code
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.CollectionItem
	for rows.Next() {
		var item models.CollectionItem
		if err := rows.Scan(
			&item.CollectionID,
			&item.StickerCode,
			&item.HaveCount,
			&item.UpdatedAt,
			&item.Sticker.Code,
			&item.Sticker.Team,
			&item.Sticker.Category,
			&item.Sticker.IsSpecial,
			&item.Sticker.Position,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *CollectionService) GetUserCollection(ctx context.Context, requesterID, targetUserID int64) ([]models.CollectionItem, models.CollectionStats, error) {
	var isPublic bool
	if err := s.db.QueryRowContext(ctx, `SELECT is_public FROM users WHERE id = $1`, targetUserID).Scan(&isPublic); err != nil {
		return nil, models.CollectionStats{}, err
	}
	if requesterID != targetUserID && !isPublic {
		return nil, models.CollectionStats{}, ErrPrivateCollection
	}
	items, err := s.GetCollection(ctx, targetUserID)
	if err != nil {
		return nil, models.CollectionStats{}, err
	}
	return items, CalculateCollectionStats(items), nil
}

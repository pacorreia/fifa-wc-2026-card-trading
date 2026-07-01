package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/models"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/ws"
)

type TradeService struct {
	db        *sql.DB
	logger    *slog.Logger
	publisher EventPublisher
}

func NewTradeService(db *sql.DB, logger *slog.Logger, publisher EventPublisher) *TradeService {
	return &TradeService{db: db, logger: logger, publisher: publisher}
}

func ComputeTradeMatch(username string, userID int64, theyHaveYouNeed, youHaveTheyNeed []models.Sticker) models.TradeMatch {
	return models.TradeMatch{
		UserID:          userID,
		Username:        username,
		TheyHaveYouNeed: theyHaveYouNeed,
		YouHaveTheyNeed: youHaveTheyNeed,
		MatchScore:      len(theyHaveYouNeed) + len(youHaveTheyNeed),
	}
}

func (s *TradeService) FindMatches(ctx context.Context, userID int64) ([]models.TradeMatch, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, username FROM users WHERE id <> $1 AND is_public = true ORDER BY username`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var matches []models.TradeMatch
	for rows.Next() {
		var otherID int64
		var username string
		if err := rows.Scan(&otherID, &username); err != nil {
			return nil, err
		}
		match, err := s.computeMatch(ctx, userID, otherID, username)
		if err != nil {
			return nil, err
		}
		if match.MatchScore > 0 {
			matches = append(matches, match)
		}
	}
	return matches, rows.Err()
}

func (s *TradeService) FindMatchWithUser(ctx context.Context, userID, otherUserID int64) (models.TradeMatch, error) {
	var username string
	if err := s.db.QueryRowContext(ctx, `SELECT username FROM users WHERE id = $1 AND is_public = true`, otherUserID).Scan(&username); err != nil {
		return models.TradeMatch{}, err
	}
	return s.computeMatch(ctx, userID, otherUserID, username)
}

func (s *TradeService) computeMatch(ctx context.Context, userID, otherUserID int64, username string) (models.TradeMatch, error) {
	theyHaveYouNeed, err := s.queryMatchSide(ctx, userID, otherUserID, true)
	if err != nil {
		return models.TradeMatch{}, err
	}
	youHaveTheyNeed, err := s.queryMatchSide(ctx, userID, otherUserID, false)
	if err != nil {
		return models.TradeMatch{}, err
	}
	return ComputeTradeMatch(username, otherUserID, theyHaveYouNeed, youHaveTheyNeed), nil
}

func (s *TradeService) queryMatchSide(ctx context.Context, userID, otherUserID int64, otherHasDuplicates bool) ([]models.Sticker, error) {
	query := `
        SELECT s.code, s.team, s.category, s.is_special, s.position
        FROM stickers s
        JOIN collections primary_c ON primary_c.user_id = $1
        JOIN collections secondary_c ON secondary_c.user_id = $2
        LEFT JOIN collection_items primary_ci ON primary_ci.collection_id = primary_c.id AND primary_ci.sticker_code = s.code
        LEFT JOIN collection_items secondary_ci ON secondary_ci.collection_id = secondary_c.id AND secondary_ci.sticker_code = s.code
        WHERE COALESCE(primary_ci.have_count, 0) = 0 AND COALESCE(secondary_ci.have_count, 0) > 1
        ORDER BY s.code`
	if !otherHasDuplicates {
		query = `
        SELECT s.code, s.team, s.category, s.is_special, s.position
        FROM stickers s
        JOIN collections primary_c ON primary_c.user_id = $1
        JOIN collections secondary_c ON secondary_c.user_id = $2
        LEFT JOIN collection_items primary_ci ON primary_ci.collection_id = primary_c.id AND primary_ci.sticker_code = s.code
        LEFT JOIN collection_items secondary_ci ON secondary_ci.collection_id = secondary_c.id AND secondary_ci.sticker_code = s.code
        WHERE COALESCE(primary_ci.have_count, 0) > 1 AND COALESCE(secondary_ci.have_count, 0) = 0
        ORDER BY s.code`
	}

	rows, err := s.db.QueryContext(ctx, query, userID, otherUserID)
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

func (s *TradeService) CreateTrade(ctx context.Context, proposerID, responderID int64, offeredCodes, requestedCodes []string) (models.Trade, error) {
	if proposerID == responderID {
		return models.Trade{}, errors.New("cannot trade with yourself")
	}
	offeredCodes = uniqueCodes(offeredCodes)
	requestedCodes = uniqueCodes(requestedCodes)
	if len(offeredCodes) == 0 || len(requestedCodes) == 0 {
		return models.Trade{}, errors.New("offered and requested sticker lists are required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Trade{}, err
	}
	defer tx.Rollback()

	var responderUsername string
	if err := tx.QueryRowContext(ctx, `SELECT username FROM users WHERE id = $1`, responderID).Scan(&responderUsername); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Trade{}, errors.New("responder not found")
		}
		return models.Trade{}, err
	}

	for _, code := range offeredCodes {
		if err := validateTradeOwnership(ctx, tx, proposerID, code, 2); err != nil {
			return models.Trade{}, fmt.Errorf("offered %s: %w", code, err)
		}
	}
	for _, code := range requestedCodes {
		if err := validateTradeOwnership(ctx, tx, responderID, code, 2); err != nil {
			return models.Trade{}, fmt.Errorf("requested %s: %w", code, err)
		}
	}

	var trade models.Trade
	err = tx.QueryRowContext(ctx, `
        INSERT INTO trades (proposer_id, responder_id, status)
        VALUES ($1, $2, 'pending')
        RETURNING id, proposer_id, responder_id, status, created_at, updated_at
    `, proposerID, responderID).Scan(&trade.ID, &trade.ProposerID, &trade.ResponderID, &trade.Status, &trade.CreatedAt, &trade.UpdatedAt)
	if err != nil {
		return models.Trade{}, err
	}

	for _, code := range offeredCodes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO trade_items (trade_id, sticker_code, direction, quantity) VALUES ($1, $2, 'offer', 1)`, trade.ID, code); err != nil {
			return models.Trade{}, err
		}
	}
	for _, code := range requestedCodes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO trade_items (trade_id, sticker_code, direction, quantity) VALUES ($1, $2, 'request', 1)`, trade.ID, code); err != nil {
			return models.Trade{}, err
		}
	}

	if err := createNotification(ctx, tx, responderID, ws.EventTradeRequestReceived, fmt.Sprintf("New trade request from user %d", proposerID)); err != nil {
		return models.Trade{}, err
	}

	if err := tx.Commit(); err != nil {
		return models.Trade{}, err
	}

	fullTrade, err := s.GetTradeByID(ctx, proposerID, trade.ID)
	if err != nil {
		return models.Trade{}, err
	}
	if s.publisher != nil {
		if evt, evtErr := ws.NewEvent(ws.EventTradeRequestReceived, fullTrade); evtErr == nil {
			_ = s.publisher.PublishToUser(responderID, evt)
		} else {
			s.logger.Error("create trade.request.received event", "error", evtErr)
		}
		_ = publishUnreadCount(ctx, s.db, s.publisher, s.logger, responderID)
	}
	return fullTrade, nil
}

func (s *TradeService) ListTrades(ctx context.Context, userID int64) ([]models.Trade, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT t.id, t.proposer_id, pu.username, t.responder_id, ru.username, t.status, t.created_at, t.updated_at
        FROM trades t
        JOIN users pu ON pu.id = t.proposer_id
        JOIN users ru ON ru.id = t.responder_id
        WHERE t.proposer_id = $1 OR t.responder_id = $1
        ORDER BY t.created_at DESC
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []models.Trade
	for rows.Next() {
		var trade models.Trade
		if err := rows.Scan(&trade.ID, &trade.ProposerID, &trade.ProposerUsername, &trade.ResponderID, &trade.ResponderUsername, &trade.Status, &trade.CreatedAt, &trade.UpdatedAt); err != nil {
			return nil, err
		}
		items, err := s.listTradeItems(ctx, trade.ID)
		if err != nil {
			return nil, err
		}
		trade.Items = items
		trades = append(trades, trade)
	}
	return trades, rows.Err()
}

func (s *TradeService) GetTradeByID(ctx context.Context, userID, tradeID int64) (models.Trade, error) {
	var trade models.Trade
	err := s.db.QueryRowContext(ctx, `
        SELECT t.id, t.proposer_id, pu.username, t.responder_id, ru.username, t.status, t.created_at, t.updated_at
        FROM trades t
        JOIN users pu ON pu.id = t.proposer_id
        JOIN users ru ON ru.id = t.responder_id
        WHERE t.id = $1 AND (t.proposer_id = $2 OR t.responder_id = $2)
    `, tradeID, userID).Scan(&trade.ID, &trade.ProposerID, &trade.ProposerUsername, &trade.ResponderID, &trade.ResponderUsername, &trade.Status, &trade.CreatedAt, &trade.UpdatedAt)
	if err != nil {
		return models.Trade{}, err
	}
	items, err := s.listTradeItems(ctx, trade.ID)
	if err != nil {
		return models.Trade{}, err
	}
	trade.Items = items
	return trade, nil
}

func (s *TradeService) listTradeItems(ctx context.Context, tradeID int64) ([]models.TradeItem, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT ti.id, ti.trade_id, ti.sticker_code, ti.direction, ti.quantity,
               s.code, s.team, s.category, s.is_special, s.position
        FROM trade_items ti
        JOIN stickers s ON s.code = ti.sticker_code
        WHERE ti.trade_id = $1
        ORDER BY ti.direction, ti.sticker_code
    `, tradeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.TradeItem
	for rows.Next() {
		var item models.TradeItem
		if err := rows.Scan(&item.ID, &item.TradeID, &item.StickerCode, &item.Direction, &item.Quantity, &item.Sticker.Code, &item.Sticker.Team, &item.Sticker.Category, &item.Sticker.IsSpecial, &item.Sticker.Position); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *TradeService) RespondTrade(ctx context.Context, userID, tradeID int64, response string) (models.Trade, error) {
	response = strings.ToLower(strings.TrimSpace(response))
	if response != "accepted" && response != "rejected" {
		return models.Trade{}, errors.New("response must be accepted or rejected")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return models.Trade{}, err
	}
	defer tx.Rollback()

	var proposerID int64
	var status string
	err = tx.QueryRowContext(ctx, `SELECT proposer_id, status FROM trades WHERE id = $1 AND responder_id = $2`, tradeID, userID).Scan(&proposerID, &status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.Trade{}, errors.New("trade not found")
		}
		return models.Trade{}, err
	}
	if status != "pending" {
		return models.Trade{}, errors.New("trade already processed")
	}

	if _, err := tx.ExecContext(ctx, `UPDATE trades SET status = $1, updated_at = NOW() WHERE id = $2`, response, tradeID); err != nil {
		return models.Trade{}, err
	}
	eventType := ws.EventTradeRequestAccepted
	if response == "rejected" {
		eventType = ws.EventTradeRequestRejected
	}
	if err := createNotification(ctx, tx, proposerID, eventType, fmt.Sprintf("Trade %d was %s", tradeID, response)); err != nil {
		return models.Trade{}, err
	}
	if err := tx.Commit(); err != nil {
		return models.Trade{}, err
	}

	trade, err := s.GetTradeByID(ctx, userID, tradeID)
	if err != nil {
		return models.Trade{}, err
	}
	if s.publisher != nil {
		if evt, evtErr := ws.NewEvent(eventType, trade); evtErr == nil {
			_ = s.publisher.PublishToUser(proposerID, evt)
		} else {
			s.logger.Error("create trade response event", "error", evtErr)
		}
		_ = publishUnreadCount(ctx, s.db, s.publisher, s.logger, proposerID)
	}
	return trade, nil
}

func uniqueCodes(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToUpper(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func validateTradeOwnership(ctx context.Context, tx *sql.Tx, userID int64, stickerCode string, minimumCount int) error {
	var count int
	err := tx.QueryRowContext(ctx, `
        SELECT COALESCE(ci.have_count, 0)
        FROM collections c
        LEFT JOIN collection_items ci ON ci.collection_id = c.id AND ci.sticker_code = $2
        WHERE c.user_id = $1
    `, userID, stickerCode).Scan(&count)
	if err != nil {
		return err
	}
	if count < minimumCount {
		return fmt.Errorf("user does not have enough copies of %s", stickerCode)
	}
	return nil
}

func createNotification(ctx context.Context, tx *sql.Tx, userID int64, notificationType, message string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO notifications (user_id, type, message) VALUES ($1, $2, $3)`, userID, notificationType, message)
	return err
}

func publishUnreadCount(ctx context.Context, database *sql.DB, publisher EventPublisher, logger *slog.Logger, userID int64) error {
	var unread int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND is_read = false`, userID).Scan(&unread); err != nil {
		return err
	}
	evt, err := ws.NewEvent(ws.EventNotificationUnread, map[string]int{"count": unread})
	if err != nil {
		logger.Error("create notification.unread.count event", "error", err)
		return err
	}
	return publisher.PublishToUser(userID, evt)
}

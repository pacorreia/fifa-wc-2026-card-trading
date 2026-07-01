package ws

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/models"
)

const (
	EventCollectionUpdated    = "collection.updated"
	EventStatsUpdated         = "stats.updated"
	EventTradeMatchFound      = "trade.match.found"
	EventTradeRequestReceived = "trade.request.received"
	EventTradeRequestAccepted = "trade.request.accepted"
	EventTradeRequestRejected = "trade.request.rejected"
	EventNotificationUnread   = "notification.unread.count"
	EventVersion              = "v1"
)

func NewEvent(eventType string, payload any) (models.Event, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return models.Event{}, fmt.Errorf("marshal event payload for %q: %w", eventType, err)
	}
	return models.Event{
		Type:      eventType,
		Version:   EventVersion,
		Timestamp: time.Now().UTC(),
		Payload:   raw,
	}, nil
}

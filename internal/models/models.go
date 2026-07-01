package models

import (
	"encoding/json"
	"time"
)

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email,omitempty"`
	PasswordHash string    `json:"-"`
	IsPublic     bool      `json:"is_public"`
	CreatedAt    time.Time `json:"created_at"`
}

type Sticker struct {
	Code      string `json:"code"`
	Team      string `json:"team"`
	Category  string `json:"category"`
	IsSpecial bool   `json:"is_special"`
	Position  string `json:"position"`
}

type Collection struct {
	ID        int64            `json:"id"`
	UserID    int64            `json:"user_id"`
	UpdatedAt time.Time        `json:"updated_at"`
	Items     []CollectionItem `json:"items"`
}

type CollectionItem struct {
	CollectionID int64     `json:"collection_id"`
	StickerCode  string    `json:"sticker_code"`
	HaveCount    int       `json:"have_count"`
	UpdatedAt    time.Time `json:"updated_at"`
	Sticker      Sticker   `json:"sticker"`
}

type TeamCompletion struct {
	Team    string  `json:"team"`
	Owned   int     `json:"owned"`
	Total   int     `json:"total"`
	Percent float64 `json:"percent"`
}

type CollectionStats struct {
	Total          int              `json:"total"`
	Owned          int              `json:"owned"`
	Missing        int              `json:"missing"`
	Duplicates     int              `json:"duplicates"`
	OverallPercent float64          `json:"overall_percent"`
	SpecialsOwned  int              `json:"specials_owned"`
	SpecialsTotal  int              `json:"specials_total"`
	SpecialsPct    float64          `json:"specials_percent"`
	TeamStats      []TeamCompletion `json:"team_stats"`
}

type Trade struct {
	ID                int64       `json:"id"`
	ProposerID        int64       `json:"proposer_id"`
	ProposerUsername  string      `json:"proposer_username,omitempty"`
	ResponderID       int64       `json:"responder_id"`
	ResponderUsername string      `json:"responder_username,omitempty"`
	Status            string      `json:"status"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
	Items             []TradeItem `json:"items"`
}

type TradeItem struct {
	ID          int64   `json:"id"`
	TradeID     int64   `json:"trade_id"`
	StickerCode string  `json:"sticker_code"`
	Direction   string  `json:"direction"`
	Quantity    int     `json:"quantity"`
	Sticker     Sticker `json:"sticker"`
}

type TradeMatch struct {
	UserID          int64     `json:"user_id"`
	Username        string    `json:"username"`
	TheyHaveYouNeed []Sticker `json:"they_have_you_need"`
	YouHaveTheyNeed []Sticker `json:"you_have_they_need"`
	MatchScore      int       `json:"match_score"`
}

type Notification struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	IsRead    bool      `json:"is_read"`
	CreatedAt time.Time `json:"created_at"`
}

type Event struct {
	Type      string          `json:"type"`
	Version   string          `json:"version"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

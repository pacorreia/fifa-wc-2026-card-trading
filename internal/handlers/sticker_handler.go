package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/models"
)

type StickerHandler struct {
	db *sql.DB
}

func NewStickerHandler(db *sql.DB) *StickerHandler {
	return &StickerHandler{db: db}
}

func (h *StickerHandler) List(w http.ResponseWriter, r *http.Request) {
	team := strings.TrimSpace(strings.ToUpper(r.URL.Query().Get("team")))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	special := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("special")))

	query := `SELECT code, team, category, is_special, position FROM stickers WHERE 1=1`
	args := make([]any, 0, 3)
	idx := 1
	if team != "" {
		query += fmt.Sprintf(` AND team = $%d`, idx)
		args = append(args, team)
		idx++
	}
	if category != "" {
		query += fmt.Sprintf(` AND category = $%d`, idx)
		args = append(args, category)
		idx++
	}
	if special == "true" || special == "false" {
		query += fmt.Sprintf(` AND is_special = $%d`, idx)
		args = append(args, special == "true")
	}
	query += ` ORDER BY team, code`

	rows, err := h.db.QueryContext(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	stickers := make([]models.Sticker, 0)
	for rows.Next() {
		var sticker models.Sticker
		if err := rows.Scan(&sticker.Code, &sticker.Team, &sticker.Category, &sticker.IsSpecial, &sticker.Position); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		stickers = append(stickers, sticker)
	}
	writeJSON(w, http.StatusOK, map[string]any{"stickers": stickers})
}

func (h *StickerHandler) Get(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(mux.Vars(r)["code"])
	var sticker models.Sticker
	err := h.db.QueryRowContext(r.Context(), `SELECT code, team, category, is_special, position FROM stickers WHERE code = $1`, code).
		Scan(&sticker.Code, &sticker.Team, &sticker.Category, &sticker.IsSpecial, &sticker.Position)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "sticker not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sticker)
}

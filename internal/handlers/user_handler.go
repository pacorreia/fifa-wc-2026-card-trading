package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/auth"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/services"
)

type UserHandler struct {
	db                *sql.DB
	collectionService *services.CollectionService
}

func NewUserHandler(db *sql.DB, collectionService *services.CollectionService) *UserHandler {
	return &UserHandler{db: db, collectionService: collectionService}
}

func (h *UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	rows, err := h.db.QueryContext(r.Context(), `SELECT id, username, is_public, created_at FROM users WHERE id <> $1 ORDER BY username`, claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	type userResponse struct {
		ID        int64     `json:"id"`
		Username  string    `json:"username"`
		IsPublic  bool      `json:"is_public"`
		CreatedAt time.Time `json:"created_at"`
	}
	users := make([]userResponse, 0)
	for rows.Next() {
		var user userResponse
		if err := rows.Scan(&user.ID, &user.Username, &user.IsPublic, &user.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		users = append(users, user)
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (h *UserHandler) GetCollection(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	targetID, ok := ParseInt64Param(w, r, "id")
	if !ok {
		return
	}
	items, stats, err := h.collectionService.GetUserCollection(r.Context(), claims.UserID, targetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		if errors.Is(err, services.ErrPrivateCollection) {
			writeError(w, http.StatusForbidden, "collection is private")
			return
		}
		slog.Default().Error("GetCollection", "user_id", targetID, "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "stats": stats})
}

package handlers

import (
	"database/sql"
	"net/http"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/auth"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/models"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/services"
)

type NotificationHandler struct {
	db        *sql.DB
	publisher services.EventPublisher
}

func NewNotificationHandler(db *sql.DB, publisher services.EventPublisher) *NotificationHandler {
	return &NotificationHandler{db: db, publisher: publisher}
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	rows, err := h.db.QueryContext(r.Context(), `
        SELECT id, user_id, type, message, is_read, created_at
        FROM notifications
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 100
    `, claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	notifications := make([]models.Notification, 0)
	for rows.Next() {
		var notification models.Notification
		if err := rows.Scan(&notification.ID, &notification.UserID, &notification.Type, &notification.Message, &notification.IsRead, &notification.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		notifications = append(notifications, notification)
	}
	writeJSON(w, http.StatusOK, map[string]any{"notifications": notifications})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	id, ok := ParseInt64Param(w, r, "id")
	if !ok {
		return
	}
	result, err := h.db.ExecContext(r.Context(), `UPDATE notifications SET is_read = true WHERE id = $1 AND user_id = $2`, id, claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		writeError(w, http.StatusNotFound, "notification not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "read"})
}

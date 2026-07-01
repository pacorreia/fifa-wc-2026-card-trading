package handlers

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/auth"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/ws"
)

type WSHandler struct {
	hub          *ws.Hub
	tokenManager *auth.TokenManager
	upgrader     websocket.Upgrader
	logger       *slog.Logger
}

func NewWSHandler(hub *ws.Hub, tokenManager *auth.TokenManager, allowedOrigins []string, logger *slog.Logger) *WSHandler {
	originSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		originSet[strings.TrimSpace(origin)] = struct{}{}
	}
	return &WSHandler{
		hub:          hub,
		tokenManager: tokenManager,
		logger:       logger,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				_, ok := originSet[origin]
				return ok
			},
		},
	}
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	claims, err := auth.AuthenticateWebSocket(r, h.tokenManager)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	client := ws.NewClient(h.hub, conn, h.logger, claims.UserID)
	if err := h.hub.Register(client); err != nil {
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, err.Error()))
		_ = conn.Close()
		return
	}

	go client.WritePump()
	go client.ReadPump()
}

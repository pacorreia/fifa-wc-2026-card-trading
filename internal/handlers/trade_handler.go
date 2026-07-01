package handlers

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/auth"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/services"
)

type TradeHandler struct {
	service *services.TradeService
}

func NewTradeHandler(service *services.TradeService) *TradeHandler {
	return &TradeHandler{service: service}
}

func (h *TradeHandler) GetMatches(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	matches, err := h.service.FindMatches(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"matches": matches})
}

func (h *TradeHandler) GetMatchWithUser(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	userID, ok := ParseInt64Param(w, r, "userId")
	if !ok {
		return
	}
	match, err := h.service.FindMatchWithUser(r.Context(), claims.UserID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, match)
}

func (h *TradeHandler) CreateTrade(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	var input struct {
		ResponderID    int64    `json:"responder_id"`
		OfferedCodes   []string `json:"offered_codes"`
		RequestedCodes []string `json:"requested_codes"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	trade, err := h.service.CreateTrade(r.Context(), claims.UserID, input.ResponderID, input.OfferedCodes, input.RequestedCodes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, trade)
}

func (h *TradeHandler) ListTrades(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	trades, err := h.service.ListTrades(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"trades": trades})
}

func (h *TradeHandler) RespondTrade(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	tradeID, ok := ParseInt64Param(w, r, "id")
	if !ok {
		return
	}
	var input struct {
		Response string `json:"response"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	trade, err := h.service.RespondTrade(r.Context(), claims.UserID, tradeID, input.Response)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, trade)
}

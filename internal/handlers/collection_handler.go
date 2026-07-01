package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/auth"
	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/services"
)

type CollectionHandler struct {
	service *services.CollectionService
}

func NewCollectionHandler(service *services.CollectionService) *CollectionHandler {
	return &CollectionHandler{service: service}
}

func (h *CollectionHandler) GetMine(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	items, err := h.service.GetCollection(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *CollectionHandler) UpdateItem(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	code := strings.ToUpper(mux.Vars(r)["code"])
	var input struct {
		HaveCount int `json:"have_count"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := h.service.UpdateItem(r.Context(), claims.UserID, code, input.HaveCount)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"item": item})
}

func (h *CollectionHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	stats, err := h.service.GetStats(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *CollectionHandler) GetMissing(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	stickers, err := h.service.GetMissing(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stickers": stickers})
}

func (h *CollectionHandler) GetDuplicates(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	items, err := h.service.GetDuplicates(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func ParseInt64Param(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	raw := mux.Vars(r)[name]
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid identifier")
		return 0, false
	}
	return value, true
}

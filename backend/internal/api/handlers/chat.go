package handlers

import (
	"fmt"
	"net/http"

	"github.com/apa/backend/internal/application"
	"github.com/apa/backend/internal/application/chat"
	"github.com/apa/backend/internal/domain"
)

type chatRequest struct {
	Message string `json:"message"`
}

func (h *Handlers) Chat(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	var req chatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	reply, err := h.deps.Chat.Handle(r.Context(), actor, req.Message)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	if reply == nil {
		reply = &chatsvc.Reply{Text: "I could not process that.", Action: "info"}
	}
	writeData(w, http.StatusOK, reply)
}

func (h *Handlers) ChatHistory(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	dayParam := r.URL.Query().Get("day")
	var messages []*application.ChatMessage
	if dayParam != "" {
		messages, err = h.deps.Chat.HistoryOnDay(r.Context(), actor, dayParam)
	} else {
		messages, err = h.deps.Chat.History(r.Context(), actor, queryInt(r, "limit", 200))
	}
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, messages)
}

func (h *Handlers) ChatHistoryDays(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	days, err := h.deps.Chat.Days(r.Context(), actor)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, days)
}

func (h *Handlers) ListEvents(w http.ResponseWriter, r *http.Request) {
	id, ok := h.identity(r)
	if !ok {
		writeError(w, r, h.deps.Log, domain.ErrUnauthorized)
		return
	}
	if !id.Role.CanApprove() {
		writeError(w, r, h.deps.Log, fmt.Errorf("%w: گزارش رویدادها فقط برای مدیران مجاز است", domain.ErrForbidden))
		return
	}
	orgID, err := parseUUID(id.OrgID)
	if err != nil {
		writeError(w, r, h.deps.Log, domain.ErrUnauthorized)
		return
	}
	events, err := h.deps.Bus.ListRecent(
		r.Context(),
		orgID,
		r.URL.Query().Get("entityType"),
		r.URL.Query().Get("entityId"),
		queryInt(r, "limit", 100),
	)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, events)
}

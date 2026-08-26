package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/apa/backend/internal/domain"
)

func (h *Handlers) ListDocuments(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	docs, err := h.deps.Documents.List(r.Context(), actor, queryInt(r, "limit", 100))
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, docs)
}

func (h *Handlers) GetDocument(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	docID, err := parseUUID(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, h.deps.Log, domain.Invalid("id", "شناسه سند نامعتبر است"))
		return
	}
	doc, err := h.deps.Documents.Get(r.Context(), actor, docID)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	writeData(w, http.StatusOK, doc)
}

type createDocumentRequest struct {
	TaskID  string `json:"taskId"`
	Content string `json:"content"`
}

func (h *Handlers) CreateDocument(w http.ResponseWriter, r *http.Request) {
	actor, err := h.actorUser(r)
	if err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	var req createDocumentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, r, h.deps.Log, err)
		return
	}
	var taskID *uuid.UUID
	if req.TaskID != "" {
		parsed, perr := parseUUID(req.TaskID)
		if perr != nil {
			writeError(w, r, h.deps.Log, domain.Invalid("taskId", "شناسه وظیفه نامعتبر است"))
			return
		}
		taskID = &parsed
	}
	result, cerr := h.deps.Documents.CreateAndGenerate(r.Context(), actor, taskID, req.Content)
	if cerr != nil {
		writeError(w, r, h.deps.Log, cerr)
		return
	}
	writeData(w, http.StatusCreated, result)
}

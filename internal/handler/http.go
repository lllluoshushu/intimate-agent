package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"intimate-agent/internal/agent"
	"intimate-agent/internal/models"
)

// Handler provides HTTP endpoints for the agent runtime.
type Handler struct {
	runtime *agent.Runtime
}

// NewHandler creates a new HTTP handler.
func NewHandler(runtime *agent.Runtime) *Handler {
	return &Handler{runtime: runtime}
}

// RegisterRoutes sets up the HTTP routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/chat", h.handleChat)
	mux.HandleFunc("/status/", h.handleStatus)
	mux.HandleFunc("/health", h.handleHealth)
}

func (h *Handler) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req models.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required")
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	reply, trace, err := h.runtime.ProcessMessage(r.Context(), req.UserID, req.Message)
	if err != nil {
		log.Printf("[ERROR] ProcessMessage failed for user %s: %v", req.UserID, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	status, _ := h.runtime.GetStatus(req.UserID)
	var sessionState models.SessionState
	if status != nil {
		sessionState = h.runtime.GetSession(req.UserID)
	}

	resp := models.ChatResponse{
		Reply:   reply,
		Trace:   trace,
		Session: sessionState,
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract user_id from path: /status/{user_id}
	userID := r.URL.Path[len("/status/"):]
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user_id is required in path")
		return
	}

	status, err := h.runtime.GetStatus(userID)
	if err != nil {
		log.Printf("[ERROR] GetStatus failed for user %s: %v", userID, err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

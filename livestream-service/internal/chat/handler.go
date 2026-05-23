package chat

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/smartraysam/livestream-service/internal/db"
	"github.com/smartraysam/livestream-service/internal/middleware"
	"github.com/smartraysam/livestream-service/internal/stream"
	"github.com/smartraysam/livestream-service/pkg/api"
)

type Handler struct {
	store   *db.Store
	streams *stream.Service
	hubs    *HubManager
	up      websocket.Upgrader
}

func NewHandler(store *db.Store, streams *stream.Service, hubs *HubManager) *Handler {
	return &Handler{
		store:   store,
		streams: streams,
		hubs:    hubs,
		up:      websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
	}
}

// Connect upgrades to a stream chat websocket.
// @Summary      Connect chat websocket
// @Description  Upgrades HTTP to websocket and subscribes user to stream chat.
// @Tags         Chat
// @Security     BearerAuth
// @Param        id   path      string  true  "Stream ID"
// @Success      101  {string}  string  "Switching Protocols"
// @Router       /streams/{id}/chat [get]
func (h *Handler) Connect(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	streamID := chi.URLParam(r, "id")
	st, err := h.streams.Get(r.Context(), streamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "stream_not_found")
		return
	}

	conn, err := h.up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	cl := &client{conn: conn, userID: user.UserID}
	hub := h.hubs.GetOrCreate(streamID, st.StreamType == "private", st.CreatorID)
	hub.register <- cl

	history, _ := h.store.ChatHistory(r.Context(), streamID, 50)
	_ = conn.WriteJSON(history)

	for {
		var in Message
		if err := conn.ReadJSON(&in); err != nil {
			hub.unregister <- cl
			return
		}
		in.UserID = user.UserID
		in.Username = user.Username
		in.SentAt = time.Now().UTC()
		if in.Type == "" {
			in.Type = "message"
		}
		_ = h.store.PutChatMessage(r.Context(), streamID, map[string]interface{}{
			"type":       in.Type,
			"user_id":    in.UserID,
			"username":   in.Username,
			"body":       in.Body,
			"amount_usd": in.AmountUSD,
			"sent_at":    in.SentAt,
		})
		hub.broadcast <- in
	}
}

// History returns last 50 chat messages.
// @Summary      Get chat history
// @Description  Returns recent chat messages for a stream.
// @Tags         Chat
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Router       /streams/{id}/chat/history [get]
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	items, err := h.store.ChatHistory(r.Context(), chi.URLParam(r, "id"), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.Response{Data: items})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{Error: msg})
}

package stream

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/livestream-service/internal/middleware"
	"github.com/yourorg/livestream-service/pkg/api"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// CreateStream creates a new broadcast or private live stream channel.
// @Summary      Create stream
// @Description  Creates an IVS channel. Set stream_type to "broadcast" for 1-to-many or "private" for 1-to-1.
// @Tags         Streams
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      stream.CreateStreamRequest  true  "Stream config"
// @Success      201   {object}  api.Response
// @Failure      400   {object}  api.ErrorResponse
// @Failure      401   {object}  api.ErrorResponse
// @Router       /streams [post]
func (h *Handler) CreateStream(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req CreateStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	stream, err := h.svc.Create(r.Context(), user.UserID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, http.StatusCreated, stream)
}

// ListStreams lists live broadcast streams sorted by viewer count.
// @Summary      List live streams
// @Description  Lists live broadcast streams in descending order by viewer count.
// @Tags         Streams
// @Produce      json
// @Success      200  {object}  api.Response
// @Router       /streams [get]
func (h *Handler) ListStreams(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListLive(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, http.StatusOK, items)
}

// GetStream fetches metadata for a stream.
// @Summary      Get stream
// @Description  Returns stream metadata for the given stream id.
// @Tags         Streams
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Failure      404  {object}  api.ErrorResponse
// @Router       /streams/{id} [get]
func (h *Handler) GetStream(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "stream_not_found")
		return
	}
	writeData(w, http.StatusOK, st)
}

// GetPlayback returns a playback URL if access checks pass.
// @Summary      Get playback URL
// @Description  Returns signed playback URL for viewer after invite/ticket checks.
// @Tags         Streams
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Failure      403  {object}  api.ErrorResponse
// @Router       /streams/{id}/playback [get]
func (h *Handler) GetPlayback(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	url, err := h.svc.CanViewPlayback(r.Context(), chi.URLParam(r, "id"), user.UserID, false)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeData(w, http.StatusOK, PlaybackResponse{PlaybackURL: url})
}

// UpdateStream updates mutable stream fields.
// @Summary      Update stream
// @Description  Updates title, description, paid settings.
// @Tags         Streams
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                      true  "Stream ID"
// @Param        body  body      stream.UpdateStreamRequest  true  "Update payload"
// @Success      200   {object}  api.Response
// @Failure      400   {object}  api.ErrorResponse
// @Router       /streams/{id} [patch]
func (h *Handler) UpdateStream(w http.ResponseWriter, r *http.Request) {
	var req UpdateStreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	st, err := h.svc.Update(r.Context(), chi.URLParam(r, "id"), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, http.StatusOK, st)
}

// DeleteStream removes stream metadata and IVS channel.
// @Summary      Delete stream
// @Description  Deletes IVS channel and stream metadata.
// @Tags         Streams
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Failure      404  {object}  api.ErrorResponse
// @Router       /streams/{id} [delete]
func (h *Handler) DeleteStream(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeError(w, http.StatusNotFound, "stream_not_found")
		return
	}
	writeData(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ListByCreator lists streams by creator id.
// @Summary      List creator streams
// @Description  Lists live and past streams for a creator.
// @Tags         Streams
// @Produce      json
// @Param        creator_id   path      string  true  "Creator ID"
// @Success      200          {object}  api.Response
// @Router       /streams/creator/{creator_id} [get]
func (h *Handler) ListByCreator(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListByCreator(r.Context(), chi.URLParam(r, "creator_id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeData(w, http.StatusOK, items)
}

func writeData(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.Response{Data: data, Error: nil})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{Error: msg})
}

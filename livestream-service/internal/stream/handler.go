package stream

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/smartraysam/livestream-service/internal/db"
	"github.com/smartraysam/livestream-service/internal/middleware"
	"github.com/smartraysam/livestream-service/pkg/api"
	"github.com/smartraysam/livestream-service/pkg/laravel"
)

type Handler struct {
	svc     *Service
	store   *db.Store
	laravel laravel.Client
}

func NewHandler(svc *Service, store *db.Store, laravelClient laravel.Client) *Handler {
	return &Handler{svc: svc, store: store, laravel: laravelClient}
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

// ListStreams lists broadcast streams. Pass ?status=live to filter LIVE-only.
// @Summary      List streams
// @Description  Returns all broadcast streams by default. Use ?status=live for live-only.
// @Tags         Streams
// @Produce      json
// @Param        status  query     string  false  "Filter by status (live)"
// @Success      200  {object}  api.Response
// @Router       /streams [get]
func (h *Handler) ListStreams(w http.ResponseWriter, r *http.Request) {
	var (
		items []*Stream
		err   error
	)
	if r.URL.Query().Get("status") == "live" {
		items, err = h.svc.ListLive(r.Context())
	} else {
		items, err = h.svc.ListAll(r.Context())
	}
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
	streamID := chi.URLParam(r, "id")
	st, err := h.svc.Get(r.Context(), streamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "stream_not_found")
		return
	}
	// Check ticket ownership so paid-stream buyers can view their content.
	hasTicket := h.store.HasTicket(r.Context(), streamID, user.UserID)
	if st.IsPaid && !hasTicket && user.UserID != st.CreatorID {
		resp, accessErr := h.laravel.CheckStreamAccess(r.Context(), laravel.StreamAccessRequest{
			UserID:    user.UserID,
			CreatorID: st.CreatorID,
			StreamID:  st.StreamID,
			IsPaid:    true,
		})
		if accessErr == nil && resp != nil && resp.CanAccess {
			hasTicket = true
		}
	}

	url, err := h.svc.CanViewPlayback(r.Context(), streamID, user.UserID, hasTicket)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeData(w, http.StatusOK, PlaybackResponse{PlaybackURL: url})
}

// GetIngestInfo returns ingest endpoint and stream key for creator broadcasting.
// @Summary      Get ingest info
// @Description  Returns IVS ingest endpoint and stream key for OBS/RTMPS broadcasting. Creator only.
// @Tags         Streams
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Failure      403  {object}  api.ErrorResponse
// @Router       /streams/{id}/ingest-info [get]
func (h *Handler) GetIngestInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	streamID := chi.URLParam(r, "id")
	st, err := h.svc.Get(r.Context(), streamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "stream_not_found")
		return
	}

	if user.UserID != st.CreatorID {
		writeError(w, http.StatusForbidden, "only_creator_can_view_ingest_info")
		return
	}

	ingest, err := h.svc.GetIngestInfo(r.Context(), streamID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeData(w, http.StatusOK, ingest)
}

// GetIVSStatus returns whether IVS currently reports this channel as live.
// @Summary      Get IVS channel status
// @Description  Returns real-time IVS channel live state for creator-owned stream.
// @Tags         Streams
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Failure      403  {object}  api.ErrorResponse
// @Router       /streams/{id}/ivs-status [get]
func (h *Handler) GetIVSStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	streamID := chi.URLParam(r, "id")
	st, err := h.svc.Get(r.Context(), streamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "stream_not_found")
		return
	}

	if user.UserID != st.CreatorID {
		writeError(w, http.StatusForbidden, "only_creator_can_view_ivs_status")
		return
	}

	status, err := h.svc.GetIVSStatus(r.Context(), streamID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	writeData(w, http.StatusOK, status)
}

// StartLiveBroadcast marks a stream as LIVE (creator only).
// @Summary      Start live broadcast
// @Description  Marks stream status as LIVE for creator-triggered broadcast start.
// @Tags         Streams
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Failure      403  {object}  api.ErrorResponse
// @Router       /streams/{id}/start-live [post]
func (h *Handler) StartLiveBroadcast(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	streamID := chi.URLParam(r, "id")
	st, err := h.svc.SetStatus(r.Context(), streamID, user.UserID, "LIVE")
	if err != nil {
		if err.Error() == "only_creator_can_change_status" {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := h.syncStreamToLaravel(r.Context(), st); err != nil {
		log.Printf("stream start sync warning: stream_id=%s err=%v", st.StreamID, err)
	}

	writeData(w, http.StatusOK, st)
}

// StopLiveBroadcast marks a stream as ENDED (creator only).
// @Summary      Stop live broadcast
// @Description  Marks stream status as ENDED for creator-triggered broadcast stop.
// @Tags         Streams
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Failure      403  {object}  api.ErrorResponse
// @Router       /streams/{id}/stop-live [post]
func (h *Handler) StopLiveBroadcast(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	streamID := chi.URLParam(r, "id")
	st, err := h.svc.SetStatus(r.Context(), streamID, user.UserID, "ENDED")
	if err != nil {
		if err.Error() == "only_creator_can_change_status" {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := h.syncStreamToLaravel(r.Context(), st); err != nil {
		log.Printf("stream stop sync warning: stream_id=%s err=%v", st.StreamID, err)
	}

	writeData(w, http.StatusOK, st)
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

// AccessCheck validates whether the authenticated user may access a stream.
//
// For locked (paid) streams: checks if the user has purchased a ticket.
// For free streams: asks the Laravel backend to verify that the user follows
// or subscribes to the creator.
//
// @Summary      Check stream access
// @Description  Returns whether the requesting user is allowed to watch this stream.
// @Tags         Streams
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Failure      403  {object}  api.ErrorResponse
// @Router       /streams/{id}/access [get]
func (h *Handler) AccessCheck(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	streamID := chi.URLParam(r, "id")
	st, err := h.svc.Get(r.Context(), streamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "stream_not_found")
		return
	}

	// Creator always has access to their own stream.
	if user.UserID == st.CreatorID {
		writeData(w, http.StatusOK, map[string]interface{}{
			"can_access": true,
			"reason":     "creator",
		})
		return
	}

	if st.IsPaid {
		// Locked stream: the user must have bought a ticket.
		hasTicket := h.store.HasTicket(r.Context(), streamID, user.UserID)
		if hasTicket {
			writeData(w, http.StatusOK, map[string]interface{}{
				"can_access": true,
				"reason":     "ticket_holder",
			})
			return
		}

		resp, accessErr := h.laravel.CheckStreamAccess(r.Context(), laravel.StreamAccessRequest{
			UserID:    user.UserID,
			CreatorID: st.CreatorID,
			StreamID:  st.StreamID,
			IsPaid:    true,
		})
		if accessErr != nil {
			writeError(w, http.StatusInternalServerError, "access_check_failed")
			return
		}
		if !resp.CanAccess {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"can_access":       false,
				"reason":           resp.Reason,
				"ticket_price_usd": st.TicketPriceUSD,
			})
			return
		}
		writeData(w, http.StatusOK, map[string]interface{}{
			"can_access": true,
			"reason":     resp.Reason,
		})
		return
	}

	// Free stream: user must follow or subscribe to the creator.
	resp, err := h.laravel.CheckStreamAccess(r.Context(), laravel.StreamAccessRequest{
		UserID:    user.UserID,
		CreatorID: st.CreatorID,
		StreamID:  st.StreamID,
		IsPaid:    false,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "access_check_failed")
		return
	}
	if !resp.CanAccess {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"can_access": false,
			"reason":     resp.Reason,
			"creator_id": st.CreatorID,
		})
		return
	}
	writeData(w, http.StatusOK, map[string]interface{}{
		"can_access": true,
		"reason":     resp.Reason,
	})
}

// SyncToLaravel pushes current stream metadata to the Laravel backend.
//
// @Summary      Sync stream to Laravel
// @Description  Persists stream details (title, status, pricing, ARNs) to the Laravel backend.
// @Tags         Streams
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Failure      404  {object}  api.ErrorResponse
// @Router       /streams/{id}/sync [post]
func (h *Handler) SyncToLaravel(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	streamID := chi.URLParam(r, "id")
	st, err := h.svc.Get(r.Context(), streamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "stream_not_found")
		return
	}
	if user.UserID != st.CreatorID {
		writeError(w, http.StatusForbidden, "only_creator_can_sync")
		return
	}
	resp, err := h.syncStreamToLaravel(r.Context(), st)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "sync_failed")
		return
	}
	writeData(w, http.StatusOK, resp)
}

func (h *Handler) syncStreamToLaravel(ctx context.Context, st *Stream) (*laravel.SyncStreamResponse, error) {
	return h.laravel.SyncStream(ctx, laravel.SyncStreamRequest{
		StreamID:       st.StreamID,
		CreatorID:      st.CreatorID,
		StreamType:     st.StreamType,
		Title:          st.Title,
		Description:    st.Description,
		IsPaid:         st.IsPaid,
		TicketPriceUSD: st.TicketPriceUSD,
		ChannelARN:     st.ChannelARN,
		PlaybackURL:    st.PlaybackURL,
		Status:         st.Status,
		CreatedAt:      st.CreatedAt,
	})
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

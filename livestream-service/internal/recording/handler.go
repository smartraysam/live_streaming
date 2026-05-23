package recording

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/smartraysam/livestream-service/internal/config"
	"github.com/smartraysam/livestream-service/internal/middleware"
	"github.com/smartraysam/livestream-service/internal/stream"
	"github.com/smartraysam/livestream-service/internal/ticket"
	"github.com/smartraysam/livestream-service/pkg/api"
)

type IVSEvent struct {
	Type           string `json:"type" example:"Stream Start"`
	StreamID       string `json:"stream_id" example:"1c95b9af-0f52-469b-9fa6-7d7375f26de8"`
	RecordingS3Key string `json:"recording_s3_key" example:"recordings/file.mp4"`
}

type Handler struct {
	cfg     *config.Config
	svc     *Service
	streams *stream.Service
	tickets *ticket.Service
}

func NewHandler(cfg *config.Config, svc *Service, streams *stream.Service, tickets *ticket.Service) *Handler {
	return &Handler{cfg: cfg, svc: svc, streams: streams, tickets: tickets}
}

// IVSWebhook receives IVS lifecycle events.
// @Summary      IVS webhook
// @Description  Receives stream lifecycle and recording events from IVS.
// @Tags         Recording
// @Accept       json
// @Produce      json
// @Param        X-IVS-Secret  header    string              true  "Webhook secret"
// @Param        body          body      recording.IVSEvent  true  "Event payload"
// @Success      200           {object}  api.Response
// @Failure      401           {object}  api.ErrorResponse
// @Router       /webhooks/ivs [post]
func (h *Handler) IVSWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-IVS-Secret") != h.cfg.IVSWebhookSecret {
		writeError(w, http.StatusUnauthorized, "invalid_webhook_secret")
		return
	}
	var ev IVSEvent
	if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := h.svc.HandleEvent(r.Context(), ev); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]bool{"ok": true})
}

// GetRecording returns recording URL when access is valid.
// @Summary      Get recording
// @Description  Returns CloudFront recording URL for a stream.
// @Tags         Recording
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Failure      403  {object}  api.ErrorResponse
// @Router       /streams/{id}/recording [get]
func (h *Handler) GetRecording(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	streamID := chi.URLParam(r, "id")
	st, err := h.streams.Get(r.Context(), streamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "stream_not_found")
		return
	}
	if st.IsPaid && !h.tickets.Verify(r.Context(), streamID, user.UserID) && st.CreatorID != user.UserID {
		writeError(w, http.StatusForbidden, "ticket_required")
		return
	}
	url := "https://" + h.cfg.CloudFrontDomain + "/" + st.RecordingS3Key
	writeData(w, http.StatusOK, map[string]string{"recording_url": url})
}

func writeData(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.Response{Data: data})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{Error: msg})
}

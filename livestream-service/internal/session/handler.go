package session

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/smartraysam/livestream-service/internal/middleware"
	"github.com/smartraysam/livestream-service/pkg/api"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// CreateSession creates a private one-to-one stream session.
// @Summary      Create private session
// @Description  Creates a private LOW_LATENCY stream and assigns invited viewer.
// @Tags         Sessions
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      session.CreateSessionRequest  true  "Session config"
// @Success      201   {object}  api.Response
// @Failure      400   {object}  api.ErrorResponse
// @Router       /sessions [post]
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	st, err := h.svc.Create(r.Context(), user.UserID, req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, http.StatusCreated, st)
}

// InviteViewer sends or updates viewer invite for a session.
// @Summary      Invite viewer
// @Description  Notifies a viewer about private session invite.
// @Tags         Sessions
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Session ID"
// @Success      200  {object}  api.Response
// @Router       /sessions/{id}/invite [post]
func (h *Handler) InviteViewer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ViewerID string `json:"viewer_id" example:"user_456"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := h.svc.Invite(r.Context(), chi.URLParam(r, "id"), body.ViewerID); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeOK(w, http.StatusOK, map[string]bool{"invited": true})
}

// IncomingInvites lists pending private session invites for caller.
// @Summary      List incoming invites
// @Description  Returns pending private sessions for the viewer.
// @Tags         Sessions
// @Security     BearerAuth
// @Produce      json
// @Success      200  {object}  api.Response
// @Router       /sessions/incoming [get]
func (h *Handler) IncomingInvites(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	items, err := h.svc.Incoming(r.Context(), user.UserID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, http.StatusOK, items)
}

// AcceptInvite accepts private session invite and charges payment via Laravel.
// @Summary      Accept session invite
// @Description  Charges session fee and grants playback ticket.
// @Tags         Sessions
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Session ID"
// @Success      200  {object}  api.Response
// @Failure      403  {object}  api.ErrorResponse
// @Router       /sessions/{id}/accept [post]
func (h *Handler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	if err := h.svc.Accept(r.Context(), chi.URLParam(r, "id"), user.UserID); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	writeOK(w, http.StatusOK, map[string]bool{"accepted": true})
}

// DeclineInvite declines private session invite.
// @Summary      Decline session invite
// @Description  Marks invite as declined.
// @Tags         Sessions
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Session ID"
// @Success      200  {object}  api.Response
// @Router       /sessions/{id}/decline [post]
func (h *Handler) DeclineInvite(w http.ResponseWriter, r *http.Request) {
	writeOK(w, http.StatusOK, map[string]bool{"declined": true})
}

func writeOK(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(api.Response{Data: data})
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(api.ErrorResponse{Error: msg})
}

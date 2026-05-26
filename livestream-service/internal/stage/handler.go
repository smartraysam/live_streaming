package stage

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/smartraysam/livestream-service/internal/middleware"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ──────────────────────────────────────────────────────────────────────────────
// CreateStage
// POST /api/v1/stages
// ──────────────────────────────────────────────────────────────────────────────

// CreateStage godoc
// @Summary      Create a real-time stage
// @Description  Provisions an IVS Real-Time stage. Use mode=CALL for 1-to-1, mode=BROADCAST for 1-to-many.
// @Tags         Stages (Real-Time)
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        body  body      stage.CreateStageRequest  true  "Stage config"
// @Success      201   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Router       /stages [post]
func (h *Handler) CreateStage(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req CreateStageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	st, err := h.svc.CreateStage(r.Context(), user.UserID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, http.StatusCreated, st)
}

// ──────────────────────────────────────────────────────────────────────────────
// GetStage
// GET /api/v1/stages/{id}
// ──────────────────────────────────────────────────────────────────────────────

// GetStage godoc
// @Summary      Get stage details
// @Tags         Stages (Real-Time)
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stage ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  map[string]string
// @Router       /stages/{id} [get]
func (h *Handler) GetStage(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.UserFromContext(r.Context()); !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	st, err := h.svc.GetStage(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		code := http.StatusInternalServerError
		if strings.Contains(err.Error(), "stage_not_found") {
			code = http.StatusNotFound
		}
		writeError(w, code, err.Error())
		return
	}
	writeData(w, http.StatusOK, st)
}

// ──────────────────────────────────────────────────────────────────────────────
// ListMyStages
// GET /api/v1/stages
// ──────────────────────────────────────────────────────────────────────────────

// ListMyStages godoc
// @Summary      List stages created by the caller
// @Tags         Stages (Real-Time)
// @Security     BearerAuth
// @Produce      json
// @Success      200  {array}   map[string]interface{}
// @Failure      401  {object}  map[string]string
// @Router       /stages [get]
func (h *Handler) ListMyStages(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	stages, err := h.svc.ListMyStages(r.Context(), user.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if stages == nil {
		stages = []*Stage{}
	}
	writeData(w, http.StatusOK, stages)
}

// ──────────────────────────────────────────────────────────────────────────────
// JoinStage
// POST /api/v1/stages/{id}/join
// ──────────────────────────────────────────────────────────────────────────────

// JoinStage godoc
// @Summary      Get a participant token to join a stage
// @Description  Returns a short-lived WebRTC token. Pass it to IVSBroadcastClient.Stage on the client.
// @Tags         Stages (Real-Time)
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                  true  "Stage ID"
// @Param        body  body      stage.JoinStageRequest  false "Join options"
// @Success      200   {object}  map[string]interface{}
// @Failure      400   {object}  map[string]string
// @Failure      401   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Router       /stages/{id}/join [post]
func (h *Handler) JoinStage(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req JoinStageRequest
	_ = json.NewDecoder(r.Body).Decode(&req) // body is optional
	req.UserID = user.UserID

	tok, err := h.svc.Join(r.Context(), chi.URLParam(r, "id"), user.UserID, req)
	if err != nil {
		code := http.StatusBadRequest
		if strings.Contains(err.Error(), "stage_not_found") {
			code = http.StatusNotFound
		}
		writeError(w, code, err.Error())
		return
	}
	writeData(w, http.StatusOK, tok)
}

// ──────────────────────────────────────────────────────────────────────────────
// EndStage
// DELETE /api/v1/stages/{id}
// ──────────────────────────────────────────────────────────────────────────────

// EndStage godoc
// @Summary      End (delete) a stage
// @Description  Only the host can end a stage. All participants are disconnected.
// @Tags         Stages (Real-Time)
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stage ID"
// @Success      204
// @Failure      401  {object}  map[string]string
// @Failure      403  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Router       /stages/{id} [delete]
func (h *Handler) EndStage(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	err := h.svc.EndStage(r.Context(), chi.URLParam(r, "id"), user.UserID)
	if err != nil {
		code := http.StatusBadRequest
		if strings.Contains(err.Error(), "stage_not_found") {
			code = http.StatusNotFound
		}
		if strings.Contains(err.Error(), "only_host") {
			code = http.StatusForbidden
		}
		writeError(w, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ──────────────────────────────────────────────────────────────────────────────
// DisconnectParticipant
// DELETE /api/v1/stages/{id}/participants/{pid}
// ──────────────────────────────────────────────────────────────────────────────

// DisconnectParticipant godoc
// @Summary      Disconnect a participant (host only)
// @Tags         Stages (Real-Time)
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string                     true  "Stage ID"
// @Param        pid   path      string                     true  "Participant ID"
// @Param        body  body      stage.DisconnectRequest    false "Reason"
// @Success      204
// @Failure      401   {object}  map[string]string
// @Failure      403   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Router       /stages/{id}/participants/{pid} [delete]
func (h *Handler) DisconnectParticipant(w http.ResponseWriter, r *http.Request) {
	user, ok := middleware.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req DisconnectRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Reason == "" {
		req.Reason = "host_removed"
	}

	err := h.svc.DisconnectParticipant(
		r.Context(),
		chi.URLParam(r, "id"),
		user.UserID,
		chi.URLParam(r, "pid"),
		req.Reason,
	)
	if err != nil {
		code := http.StatusBadRequest
		if strings.Contains(err.Error(), "stage_not_found") {
			code = http.StatusNotFound
		}
		if strings.Contains(err.Error(), "only_host") {
			code = http.StatusForbidden
		}
		writeError(w, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ──────────────────────────────────────────────────────────────────────────────
// helpers (mirrors stream/handler.go pattern)
// ──────────────────────────────────────────────────────────────────────────────

func writeData(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

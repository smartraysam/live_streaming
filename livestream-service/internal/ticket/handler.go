package ticket

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/smartraysam/livestream-service/internal/middleware"
	"github.com/smartraysam/livestream-service/pkg/api"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// PurchaseTicket purchases stream ticket via Laravel.
// @Summary      Purchase ticket
// @Description  Charges viewer and grants stream ticket on success.
// @Tags         Tickets
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Failure      403  {object}  api.ErrorResponse
// @Router       /streams/{id}/ticket/purchase [post]
func (h *Handler) PurchaseTicket(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	res, err := h.svc.Purchase(r.Context(), chi.URLParam(r, "id"), user.UserID)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	writeData(w, http.StatusOK, map[string]interface{}{"ticket_granted": true, "transaction": res})
}

// VerifyTicket verifies if caller has valid stream ticket.
// @Summary      Verify ticket
// @Description  Checks if viewer owns valid stream ticket.
// @Tags         Tickets
// @Security     BearerAuth
// @Produce      json
// @Param        id   path      string  true  "Stream ID"
// @Success      200  {object}  api.Response
// @Router       /streams/{id}/ticket/verify [get]
func (h *Handler) VerifyTicket(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	has := h.svc.Verify(r.Context(), chi.URLParam(r, "id"), user.UserID)
	writeData(w, http.StatusOK, VerifyResponse{HasTicket: has})
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

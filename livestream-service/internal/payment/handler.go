package payment

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/smartraysam/livestream-service/internal/middleware"
	"github.com/smartraysam/livestream-service/pkg/api"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// TipStream initiates a tip payment through Laravel.
// @Summary      Send tip
// @Description  Charges viewer and emits tip alert event to chat.
// @Tags         Payments
// @Security     BearerAuth
// @Accept       json
// @Produce      json
// @Param        id    path      string              true  "Stream ID"
// @Param        body  body      payment.TipRequest  true  "Tip payload"
// @Success      200   {object}  api.Response
// @Failure      400   {object}  api.ErrorResponse
// @Router       /streams/{id}/tip [post]
func (h *Handler) TipStream(w http.ResponseWriter, r *http.Request) {
	user, _ := middleware.UserFromContext(r.Context())
	var req TipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	res, err := h.svc.Tip(r.Context(), chi.URLParam(r, "id"), user.UserID, req.Message, req.AmountUSD)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeData(w, http.StatusOK, res)
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

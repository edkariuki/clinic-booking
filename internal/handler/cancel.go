package handler

import (
	"fmt"
	"net/http"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

type cancelRequest struct {
	Reason string `json:"reason"`
}

// cancelAppointment handles PATCH /appointments/{id}/cancel.
func (h *Handler) cancelAppointment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		writeError(w, fmt.Errorf("%w: appointment id must be a UUID", domain.ErrInvalidInput))
		return
	}

	var req cancelRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}

	appt, err := h.svc.Cancel(r.Context(), id, req.Reason)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAppointmentResponse(appt))
}

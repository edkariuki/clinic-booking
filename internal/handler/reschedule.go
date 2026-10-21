package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

type rescheduleRequest struct {
	StartTime string  `json:"start_time"`
	DoctorID  *string `json:"doctor_id"`
}

// rescheduleAppointment handles PATCH /appointments/{id}/reschedule.
func (h *Handler) rescheduleAppointment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !uuidPattern.MatchString(id) {
		writeError(w, fmt.Errorf("%w: appointment id must be a UUID", domain.ErrInvalidInput))
		return
	}

	var req rescheduleRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}

	start, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		writeError(w, fmt.Errorf("%w: start_time is required and must be an RFC 3339 timestamp with an offset, e.g. 2026-10-26T10:00:00+03:00", domain.ErrInvalidInput))
		return
	}

	//empty doctor ID means same doc
	doctorID := ""
	if req.DoctorID != nil {
		if !uuidPattern.MatchString(*req.DoctorID) {
			writeError(w, fmt.Errorf("%w: doctor_id must be a UUID when provided", domain.ErrInvalidInput))
			return
		}
		doctorID = *req.DoctorID
	}

	appt, err := h.svc.Reschedule(r.Context(), id, doctorID, start)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAppointmentResponse(appt))
}

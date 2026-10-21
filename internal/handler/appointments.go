package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

type createAppointmentRequest struct {
	DoctorID  string `json:"doctor_id"`
	PatientID string `json:"patient_id"`
	StartTime string `json:"start_time"`
}

type appointmentResponse struct {
	ID                 string    `json:"id"`
	DoctorID           string    `json:"doctor_id"`
	PatientID          string    `json:"patient_id"`
	StartTime          time.Time `json:"start_time"`
	EndTime            time.Time `json:"end_time"`
	Status             string    `json:"status"`
	CancellationReason *string   `json:"cancellation_reason"`
	LateCancellation   bool      `json:"late_cancellation"`
}

func toAppointmentResponse(a domain.Appointment) appointmentResponse {
	return appointmentResponse{
		ID:                 a.ID,
		DoctorID:           a.DoctorID,
		PatientID:          a.PatientID,
		StartTime:          a.StartTime.In(domain.ClinicLocation),
		EndTime:            a.EndTime.In(domain.ClinicLocation),
		Status:             string(a.Status),
		CancellationReason: a.CancellationReason,
		LateCancellation:   a.LateCancellation,
	}
}

// createAppointment handles POST /appointments.
func (h *Handler) createAppointment(w http.ResponseWriter, r *http.Request) {
	var req createAppointmentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, err)
		return
	}

	if !uuidPattern.MatchString(req.DoctorID) {
		writeError(w, fmt.Errorf("%w: doctor_id must be a UUID", domain.ErrInvalidInput))
		return
	}
	start, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		writeError(w, fmt.Errorf("%w: start_time is required and must be an RFC 3339 timestamp with an offset, e.g. 2026-10-26T10:00:00+03:00", domain.ErrInvalidInput))
		return
	}

	appt, err := h.svc.Book(r.Context(), req.DoctorID, req.PatientID, start)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAppointmentResponse(appt))
}

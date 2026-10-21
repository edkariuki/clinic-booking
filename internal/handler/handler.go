package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

type Service interface {
	Availability(ctx context.Context, doctorID string, day time.Time) ([]time.Time, error)
	Book(ctx context.Context, doctorID, patientID string, start time.Time) (domain.Appointment, error)
	Cancel(ctx context.Context, id, reason string) (domain.Appointment, error)
	Reschedule(ctx context.Context, id, doctorID string, start time.Time) (domain.Appointment, error)
}

type Handler struct {
	svc Service
}

func New(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /doctors/{id}/availability", h.availability)
	mux.HandleFunc("POST /appointments", h.createAppointment)
	mux.HandleFunc("PATCH /appointments/{id}/cancel", h.cancelAppointment)
	mux.HandleFunc("PATCH /appointments/{id}/reschedule", h.rescheduleAppointment)
}

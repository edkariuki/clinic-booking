package handler

import (
	"context"
	"net/http"
	"time"
)

type Service interface {
	Availability(ctx context.Context, doctorID string, day time.Time) ([]time.Time, error)
}

type Handler struct {
	svc Service
}

func New(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /doctors/{id}/availability", h.availability)
}

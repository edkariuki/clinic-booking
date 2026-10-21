package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "error", err)
	}
}

// writeError maps error to HTTP status and JSON body
func writeError(w http.ResponseWriter, err error) {
	status := statusFor(err)
	if status == http.StatusInternalServerError {
		slog.Error("internal error", "error", err)
		writeJSON(w, status, errorResponse{Error: "internal server error"})
		return
	}
	writeJSON(w, status, errorResponse{Error: err.Error()})
}

// statusFor changes domain errors to HTTP status codes:
func statusFor(err error) int {
	switch {
	case errors.Is(err, domain.ErrInvalidInput),
		errors.Is(err, domain.ErrReasonRequired),
		errors.Is(err, domain.ErrReasonTooLong):
		return http.StatusBadRequest
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, domain.ErrSlotTaken),
		errors.Is(err, domain.ErrPatientBusy),
		errors.Is(err, domain.ErrAlreadyCancelled):
		return http.StatusConflict
	case errors.Is(err, domain.ErrInPast),
		errors.Is(err, domain.ErrTooSoon),
		errors.Is(err, domain.ErrOutsideWorkingHours),
		errors.Is(err, domain.ErrMisaligned),
		errors.Is(err, domain.ErrAlreadyStarted),
		errors.Is(err, domain.ErrRescheduleTooLate):
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

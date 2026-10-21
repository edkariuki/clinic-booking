package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

type slotResponse struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type availabilityResponse struct {
	DoctorID string         `json:"doctor_id"`
	Date     string         `json:"date"`
	Slots    []slotResponse `json:"slots"`
}

// availability handles GET /doctors/{id}/availability with date query param
func (h *Handler) availability(w http.ResponseWriter, r *http.Request) {
	doctorID := r.PathValue("id")
	if !uuidPattern.MatchString(doctorID) {
		writeError(w, fmt.Errorf("%w: doctor id must be a UUID", domain.ErrInvalidInput))
		return
	}

	dateParam := r.URL.Query().Get("date")
	if dateParam == "" {
		writeError(w, fmt.Errorf("%w: the date query parameter is required (YYYY-MM-DD)", domain.ErrInvalidInput))
		return
	}
	day, err := time.ParseInLocation("2006-01-02", dateParam, domain.ClinicLocation)
	if err != nil {
		writeError(w, fmt.Errorf("%w: date must be a valid date in the form YYYY-MM-DD", domain.ErrInvalidInput))
		return
	}

	starts, err := h.svc.Availability(r.Context(), doctorID, day)
	if err != nil {
		writeError(w, err)
		return
	}

	slots := make([]slotResponse, 0, len(starts))
	for _, s := range starts {
		start := s.In(domain.ClinicLocation)
		slots = append(slots, slotResponse{Start: start, End: start.Add(domain.SlotDuration)})
	}
	writeJSON(w, http.StatusOK, availabilityResponse{
		DoctorID: doctorID,
		Date:     day.Format("2006-01-02"),
		Slots:    slots,
	})
}

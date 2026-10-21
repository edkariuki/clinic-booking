package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

const maxPatientIDLength = 64

func normalizePatientID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || utf8.RuneCountInString(id) > maxPatientIDLength {
		return "", fmt.Errorf("%w: patient_id must be 1 to %d characters", domain.ErrInvalidInput, maxPatientIDLength)
	}
	return id, nil
}

// Book creates a confirmed appointment for patient with doc at start with service validation
func (s *Service) Book(ctx context.Context, doctorID, patientID string, start time.Time) (domain.Appointment, error) {
	patientID = strings.TrimSpace(patientID)
	if patientID == "" || utf8.RuneCountInString(patientID) > maxPatientIDLength {
		return domain.Appointment{}, fmt.Errorf("%w: patient_id must be 1 to %d characters", domain.ErrInvalidInput, maxPatientIDLength)
	}

	if _, err := s.repo.GetDoctor(ctx, doctorID); err != nil {
		return domain.Appointment{}, err
	}

	shifts, err := s.repo.ListShifts(ctx, doctorID)
	if err != nil {
		return domain.Appointment{}, err
	}

	if err := ValidateNewSlot(shifts, start, s.now()); err != nil {
		return domain.Appointment{}, err
	}

	return s.repo.CreateAppointment(ctx, doctorID, patientID, start)
}

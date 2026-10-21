package service

import (
	"context"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// UpcomingForPatient returns the patient confirmed appointments that have not bega from earliest first.
func (s *Service) UpcomingForPatient(ctx context.Context, patientID string) ([]domain.Appointment, error) {
	patientID, err := normalizePatientID(patientID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListUpcomingForPatient(ctx, patientID, s.now())
}

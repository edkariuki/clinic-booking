package service

import (
	"context"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// Cancel cancels the appointment with the given ID and validated reason
func (s *Service) Cancel(ctx context.Context, id, reason string) (domain.Appointment, error) {
	reason, err := NormalizeReason(reason)
	if err != nil {
		return domain.Appointment{}, err
	}

	return s.repo.CancelAppointment(ctx, id, reason, func(current domain.Appointment) (bool, error) {
		return CancellationOutcome(current.StartTime, s.now())
	})
}

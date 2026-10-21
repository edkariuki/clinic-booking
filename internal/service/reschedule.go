package service

import (
	"context"
	"strings"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// Reschedule moves an appointment to a new start time and, optionally, a new doc
func (s *Service) Reschedule(ctx context.Context, id, doctorID string, start time.Time) (domain.Appointment, error) {
	current, err := s.repo.GetAppointment(ctx, id)
	if err != nil {
		return domain.Appointment{}, err
	}

	target := strings.TrimSpace(doctorID)
	if target == "" {
		target = current.DoctorID
	}

	if target != current.DoctorID {
		if _, err := s.repo.GetDoctor(ctx, target); err != nil {
			return domain.Appointment{}, err
		}
	}

	shifts, err := s.repo.ListShifts(ctx, target)
	if err != nil {
		return domain.Appointment{}, err
	}

	return s.repo.RescheduleAppointment(ctx, id, target, start, func(locked domain.Appointment) error {
		now := s.now()
		if err := ValidateReschedulable(locked.StartTime, now); err != nil {
			return err
		}
		return ValidateNewSlot(shifts, start, now)
	})
}

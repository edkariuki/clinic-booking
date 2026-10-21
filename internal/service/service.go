package service

import (
	"context"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

type Repository interface {
	GetDoctor(ctx context.Context, id string) (domain.Doctor, error)
	ListShifts(ctx context.Context, doctorID string) ([]domain.Shift, error)
	ListConfirmedStarts(ctx context.Context, doctorID string, from, to time.Time) ([]time.Time, error)
}

type Service struct {
	repo Repository
	now  func() time.Time
}

func New(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now}
}

// Availability returns free 30min slot start times for a doc on given day in order
func (s *Service) Availability(ctx context.Context, doctorID string, day time.Time) ([]time.Time, error) {
	if _, err := s.repo.GetDoctor(ctx, doctorID); err != nil {
		return nil, err
	}

	shifts, err := s.repo.ListShifts(ctx, doctorID)
	if err != nil {
		return nil, err
	}

	local := day.In(domain.ClinicLocation)
	y, m, d := local.Date()
	from := time.Date(y, m, d, 0, 0, 0, 0, domain.ClinicLocation)
	to := time.Date(y, m, d+1, 0, 0, 0, 0, domain.ClinicLocation)

	taken, err := s.repo.ListConfirmedStarts(ctx, doctorID, from, to)
	if err != nil {
		return nil, err
	}

	return AvailableSlots(shifts, from, taken, s.now()), nil
}

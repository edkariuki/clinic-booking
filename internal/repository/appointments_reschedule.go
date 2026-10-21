package repository

import (
	"context"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// RescheduleAppointment moves an existing appointment to new start time and optioally a new doc.
func (r *Repository) RescheduleAppointment(
	ctx context.Context,
	id, doctorID string,
	start time.Time,
	decide func(current domain.Appointment) error,
) (domain.Appointment, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return domain.Appointment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := scanAppointment(tx.QueryRow(ctx,
		`SELECT `+appointmentColumns+` FROM appointments WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return domain.Appointment{}, mapError(err)
	}
	if current.Status == domain.StatusCancelled {
		return domain.Appointment{}, domain.ErrAlreadyCancelled
	}

	if err := decide(current); err != nil {
		return domain.Appointment{}, err
	}

	moved, err := scanAppointment(tx.QueryRow(ctx,
		`UPDATE appointments
		    SET doctor_id = $2, start_time = $3, end_time = $4, updated_at = now()
		  WHERE id = $1
		  RETURNING `+appointmentColumns,
		id, doctorID, start, start.Add(domain.SlotDuration)))
	if err != nil {
		return domain.Appointment{}, mapError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Appointment{}, mapError(err)
	}
	return moved, nil
}

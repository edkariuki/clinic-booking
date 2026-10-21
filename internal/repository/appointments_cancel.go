package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

const appointmentColumns = `id::text, doctor_id::text, patient_id, start_time, end_time,
	status, cancellation_reason, late_cancellation, created_at, updated_at`

// scanAppointment reads 1 row selected with appointmentColumns.
func scanAppointment(row pgx.Row) (domain.Appointment, error) {
	var (
		a      domain.Appointment
		status string
	)
	err := row.Scan(&a.ID, &a.DoctorID, &a.PatientID, &a.StartTime, &a.EndTime,
		&status, &a.CancellationReason, &a.LateCancellation, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return domain.Appointment{}, err
	}
	a.Status = domain.Status(status)
	return a, nil
}

// CancelAppointment cancels an appointment inside one tx
func (r *Repository) CancelAppointment(
	ctx context.Context,
	id, reason string,
	decide func(current domain.Appointment) (late bool, err error),
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

	late, err := decide(current)
	if err != nil {
		return domain.Appointment{}, err
	}

	cancelled, err := scanAppointment(tx.QueryRow(ctx,
		`UPDATE appointments
		    SET status = 'CANCELLED', cancellation_reason = $2, late_cancellation = $3, updated_at = now()
		  WHERE id = $1
		  RETURNING `+appointmentColumns, id, reason, late))
	if err != nil {
		return domain.Appointment{}, mapError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Appointment{}, mapError(err)
	}
	return cancelled, nil
}

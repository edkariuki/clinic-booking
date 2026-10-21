package repository

import (
	"context"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// CreateAppointment inserts a confirmed 30min appointment beginning at StartTime after checking that doc and patient are free.
func (r *Repository) CreateAppointment(ctx context.Context, doctorID, patientID string, start time.Time) (domain.Appointment, error) {
	var (
		a      domain.Appointment
		status string
	)
	err := r.db.QueryRow(ctx,
		`INSERT INTO appointments (doctor_id, patient_id, start_time, end_time)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id::text, doctor_id::text, patient_id, start_time, end_time,
		           status, cancellation_reason, late_cancellation, created_at, updated_at`,
		doctorID, patientID, start, start.Add(domain.SlotDuration),
	).Scan(&a.ID, &a.DoctorID, &a.PatientID, &a.StartTime, &a.EndTime,
		&status, &a.CancellationReason, &a.LateCancellation, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return domain.Appointment{}, mapError(err)
	}
	a.Status = domain.Status(status)
	return a, nil
}

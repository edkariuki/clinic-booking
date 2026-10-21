package repository

import (
	"context"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// ListUpcomingForPatient returns the patient confirmed appointments beginning from now, earliest first.
func (r *Repository) ListUpcomingForPatient(ctx context.Context, patientID string, from time.Time) ([]domain.Appointment, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+appointmentColumns+`
		   FROM appointments
		  WHERE patient_id = $1
		    AND status = 'CONFIRMED'
		    AND start_time >= $2
		  ORDER BY start_time`, patientID, from)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	appointments := []domain.Appointment{}
	for rows.Next() {
		a, err := scanAppointment(rows)
		if err != nil {
			return nil, err
		}
		appointments = append(appointments, a)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return appointments, nil
}

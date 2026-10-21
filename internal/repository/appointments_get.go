package repository

import (
	"context"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// GetAppointment returns the appointment with the given ID
func (r *Repository) GetAppointment(ctx context.Context, id string) (domain.Appointment, error) {
	a, err := scanAppointment(r.db.QueryRow(ctx,
		`SELECT `+appointmentColumns+` FROM appointments WHERE id = $1`, id))
	if err != nil {
		return domain.Appointment{}, mapError(err)
	}
	return a, nil
}

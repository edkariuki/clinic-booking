package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository struct {
	db DBTX
}

func New(db DBTX) *Repository {
	return &Repository{db: db}
}

// GetDoctor returns the doc or errors.ErrorNotFound if doc does not exsit
func (r *Repository) GetDoctor(ctx context.Context, id string) (domain.Doctor, error) {
	var d domain.Doctor
	err := r.db.QueryRow(ctx,
		`SELECT id::text, name, created_at FROM doctors WHERE id = $1`, id,
	).Scan(&d.ID, &d.Name, &d.CreatedAt)
	if err != nil {
		return domain.Doctor{}, mapError(err)
	}
	return d, nil
}

// ListShifts returns the doctor's recurring weekly shifts or empty if no shifts
func (r *Repository) ListShifts(ctx context.Context, doctorID string) ([]domain.Shift, error) {
	rows, err := r.db.Query(ctx,
		`SELECT doctor_id::text, day_of_week, start_time, end_time
		   FROM doctor_schedules
		  WHERE doctor_id = $1
		  ORDER BY day_of_week, start_time`, doctorID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	shifts := []domain.Shift{}
	for rows.Next() {
		var (
			s          domain.Shift
			dayOfWeek  int16
			start, end pgtype.Time
		)
		if err := rows.Scan(&s.DoctorID, &dayOfWeek, &start, &end); err != nil {
			return nil, err
		}
		s.DayOfWeek = time.Weekday(dayOfWeek)
		s.Start = timeOfDay(start)
		s.End = timeOfDay(end)
		shifts = append(shifts, s)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return shifts, nil
}

// ListConfirmedStarts returns the start times of the doc confirmed appointments
func (r *Repository) ListConfirmedStarts(ctx context.Context, doctorID string, from, to time.Time) ([]time.Time, error) {
	rows, err := r.db.Query(ctx,
		`SELECT start_time
		   FROM appointments
		  WHERE doctor_id = $1
		    AND status = 'CONFIRMED'
		    AND start_time >= $2
		    AND start_time <  $3
		  ORDER BY start_time`, doctorID, from, to)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	starts := []time.Time{}
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		starts = append(starts, t)
	}
	if err := rows.Err(); err != nil {
		return nil, mapError(err)
	}
	return starts, nil
}

// timeOfDay converts a Postgres TIME to duration
func timeOfDay(t pgtype.Time) time.Duration {
	return time.Duration(t.Microseconds) * time.Microsecond
}

// mapError turns database errors to domain errors
func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "22P02" {
		return domain.ErrInvalidInput
	}
	return err
}

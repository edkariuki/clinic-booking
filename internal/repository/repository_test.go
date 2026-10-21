package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL is not set; skipping database tests")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newDoctor(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx,
		`INSERT INTO doctors (name) VALUES ('Repository Test Doctor') RETURNING id::text`,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert doctor: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM appointments WHERE doctor_id = $1`, id)
		_, _ = pool.Exec(ctx, `DELETE FROM doctors WHERE id = $1`, id) // shifts cascade
	})
	return id
}

// Checks exsisting doc come back with correct ID and name
func TestGetDoctor(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	id := newDoctor(t, pool)

	got, err := repo.GetDoctor(context.Background(), id)
	if err != nil {
		t.Fatalf("GetDoctor: %v", err)
	}
	if got.ID != id || got.Name != "Repository Test Doctor" {
		t.Errorf("got %+v", got)
	}

	_, err = repo.GetDoctor(context.Background(), "00000000-0000-4000-8000-000000000000")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown doctor: got %v, want ErrNotFound", err)
	}

	_, err = repo.GetDoctor(context.Background(), "not-a-uuid")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("malformed id: got %v, want ErrInvalidInput", err)
	}
}

// checks shifts are listed in correct order
func TestListShifts(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	id := newDoctor(t, pool)
	ctx := context.Background()

	_, err := pool.Exec(ctx,
		`INSERT INTO doctor_schedules (doctor_id, day_of_week, start_time, end_time) VALUES
		   ($1, 1, '13:00', '17:00'),
		   ($1, 1, '09:00', '12:00')`, id)
	if err != nil {
		t.Fatalf("insert shifts: %v", err)
	}

	got, err := repo.ListShifts(ctx, id)
	if err != nil {
		t.Fatalf("ListShifts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 shifts, got %d", len(got))
	}

	if got[0].DayOfWeek != time.Monday || got[0].Start != 9*time.Hour || got[0].End != 12*time.Hour {
		t.Errorf("first shift = %+v, want Monday 09:00-12:00", got[0])
	}
	if got[1].Start != 13*time.Hour || got[1].End != 17*time.Hour {
		t.Errorf("second shift = %+v, want 13:00-17:00", got[1])
	}

	none, err := repo.ListShifts(ctx, "00000000-0000-4000-8000-000000000000")
	if err != nil || none == nil || len(none) != 0 {
		t.Errorf("unknown doctor: got (%v, %v), want empty non-nil slice and no error", none, err)
	}
}

// checks that only confirmed appointments are returned
func TestListConfirmedStarts(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	id := newDoctor(t, pool)
	ctx := context.Background()

	base := time.Date(2031, time.January, 6, 6, 0, 0, 0, time.UTC)

	insertConfirmed := func(patient string, start time.Time) {
		t.Helper()
		_, err := pool.Exec(ctx,
			`INSERT INTO appointments (doctor_id, patient_id, start_time, end_time)
			 VALUES ($1, $2, $3::timestamptz, $3::timestamptz + interval '30 minutes')`,
			id, patient, start)
		if err != nil {
			t.Fatalf("insert confirmed: %v", err)
		}
	}
	insertConfirmed("repo-test-a", base)
	insertConfirmed("repo-test-b", base.Add(24*time.Hour))

	_, err := pool.Exec(ctx,
		`INSERT INTO appointments (doctor_id, patient_id, start_time, end_time, status, cancellation_reason)
		 VALUES ($1, 'repo-test-c', $2::timestamptz, $2::timestamptz + interval '30 minutes', 'CANCELLED', 'test')`,
		id, base.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("insert cancelled: %v", err)
	}

	got, err := repo.ListConfirmedStarts(ctx, id, base, base.Add(12*time.Hour))
	if err != nil {
		t.Fatalf("ListConfirmedStarts: %v", err)
	}
	if len(got) != 1 || !got[0].Equal(base) {
		t.Errorf("got %v, want only the confirmed appointment at %v", got, base)
	}

	got, err = repo.ListConfirmedStarts(ctx, id, base.Add(-time.Hour), base)
	if err != nil || len(got) != 0 {
		t.Errorf("range ending at the start time: got (%v, %v), want empty", got, err)
	}
}

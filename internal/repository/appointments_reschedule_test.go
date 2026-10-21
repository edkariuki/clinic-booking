package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

func allow(domain.Appointment) error { return nil }

func stored(t *testing.T, repo *Repository, id string) domain.Appointment {
	t.Helper()
	a, err := repo.GetAppointment(context.Background(), id)
	if err != nil {
		t.Fatalf("GetAppointment: %v", err)
	}
	return a
}

// books appointment and fetches the appointment by ID
func TestGetAppointment(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	doctor := newDoctor(t, pool)
	ctx := context.Background()

	start := time.Date(2031, time.June, 2, 6, 0, 0, 0, time.UTC)
	booked, err := repo.CreateAppointment(ctx, doctor, "repo-test-get-1", start)
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}

	got, err := repo.GetAppointment(ctx, booked.ID)
	if err != nil || got.ID != booked.ID || got.DoctorID != doctor || !got.StartTime.Equal(start) {
		t.Errorf("got (%+v, %v)", got, err)
	}
	if _, err := repo.GetAppointment(ctx, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown id: got %v, want ErrNotFound", err)
	}
	if _, err := repo.GetAppointment(ctx, "not-a-uuid"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("malformed id: got %v, want ErrInvalidInput", err)
	}
}

// reschedules appointments with different paths
func TestRescheduleAppointment(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	doctorA := newDoctor(t, pool)
	doctorB := newDoctor(t, pool)
	ctx := context.Background()

	start := time.Date(2031, time.June, 3, 6, 0, 0, 0, time.UTC)
	later := start.Add(2 * time.Hour)
	booked, err := repo.CreateAppointment(ctx, doctorA, "repo-test-resched-1", start)
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}

	// Same doc, new time: same ID and patient, new times.
	got, err := repo.RescheduleAppointment(ctx, booked.ID, doctorA, later, allow)
	if err != nil {
		t.Fatalf("RescheduleAppointment: %v", err)
	}
	if got.ID != booked.ID || got.PatientID != "repo-test-resched-1" || got.Status != domain.StatusConfirmed {
		t.Errorf("unexpected appointment: %+v", got)
	}
	if got.DoctorID != doctorA || !got.StartTime.Equal(later) || !got.EndTime.Equal(later.Add(30*time.Minute)) {
		t.Errorf("want doctor A at %v, got %+v", later, got)
	}

	// The old slot is free again for someone else.
	if _, err := repo.CreateAppointment(ctx, doctorA, "repo-test-resched-2", start); err != nil {
		t.Errorf("the vacated slot should be bookable: %v", err)
	}

	// Different doc and time.
	moved := later.Add(time.Hour)
	got, err = repo.RescheduleAppointment(ctx, booked.ID, doctorB, moved, allow)
	if err != nil {
		t.Fatalf("move to another doctor: %v", err)
	}
	if got.DoctorID != doctorB || !got.StartTime.Equal(moved) {
		t.Errorf("want doctor B at %v, got %+v", moved, got)
	}
}

// checks unique indexes at work
func TestRescheduleAppointmentConflicts(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	doctorA := newDoctor(t, pool)
	doctorB := newDoctor(t, pool)
	ctx := context.Background()

	start := time.Date(2031, time.June, 4, 6, 0, 0, 0, time.UTC)
	mine, err := repo.CreateAppointment(ctx, doctorA, "repo-test-resched-3", start)
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}

	// Another patient holds doctor A at +1hr; the same patient holds doctor B at +2hr
	if _, err := repo.CreateAppointment(ctx, doctorA, "repo-test-resched-4", start.Add(time.Hour)); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := repo.CreateAppointment(ctx, doctorB, "repo-test-resched-3", start.Add(2*time.Hour)); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// The doc target slot is taken.
	_, err = repo.RescheduleAppointment(ctx, mine.ID, doctorA, start.Add(time.Hour), allow)
	if !errors.Is(err, domain.ErrSlotTaken) {
		t.Errorf("taken slot: got %v, want ErrSlotTaken", err)
	}

	// The patient already has an appointment at the target time (with another doc).
	_, err = repo.RescheduleAppointment(ctx, mine.ID, doctorA, start.Add(2*time.Hour), allow)
	if !errors.Is(err, domain.ErrPatientBusy) {
		t.Errorf("patient clash: got %v, want ErrPatientBusy", err)
	}

	// A target doc that does not exist.
	_, err = repo.RescheduleAppointment(ctx, mine.ID, "00000000-0000-4000-8000-000000000000", start.Add(3*time.Hour), allow)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown doctor: got %v, want ErrNotFound", err)
	}

	// After all those failures the original booking is untouched.
	after := stored(t, repo, mine.ID)
	if after.DoctorID != doctorA || !after.StartTime.Equal(start) || after.Status != domain.StatusConfirmed {
		t.Errorf("failed reschedules must not change the booking, got %+v", after)
	}
}

// checks the rules are run and can reject a reschedule
func TestRescheduleAppointmentErrors(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	doctor := newDoctor(t, pool)
	ctx := context.Background()

	start := time.Date(2031, time.June, 5, 6, 0, 0, 0, time.UTC)
	target := start.Add(time.Hour)
	booked, err := repo.CreateAppointment(ctx, doctor, "repo-test-resched-5", start)
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}

	// The service's rule rejects the move
	_, err = repo.RescheduleAppointment(ctx, booked.ID, doctor, target, func(domain.Appointment) error {
		return domain.ErrRescheduleTooLate
	})
	if !errors.Is(err, domain.ErrRescheduleTooLate) {
		t.Errorf("got %v, want ErrRescheduleTooLate", err)
	}
	if got := stored(t, repo, booked.ID); !got.StartTime.Equal(start) {
		t.Errorf("a rejected reschedule must not move the booking, got %v", got.StartTime)
	}

	// decide sees the locked, current row.
	var seen domain.Appointment
	_, err = repo.RescheduleAppointment(ctx, booked.ID, doctor, target, func(c domain.Appointment) error {
		seen = c
		return nil
	})
	if err != nil || seen.ID != booked.ID || !seen.StartTime.Equal(start) {
		t.Errorf("decide should receive the current appointment, saw %+v (err %v)", seen, err)
	}

	// A cancelled appointment cannot be rescheduled, and the rules do not run.
	if _, err := repo.CancelAppointment(ctx, booked.ID, "changed my mind", decideOnTime); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	called := false
	_, err = repo.RescheduleAppointment(ctx, booked.ID, doctor, target.Add(time.Hour), func(domain.Appointment) error {
		called = true
		return nil
	})
	if !errors.Is(err, domain.ErrAlreadyCancelled) {
		t.Errorf("cancelled: got %v, want ErrAlreadyCancelled", err)
	}
	if called {
		t.Error("the rules must not run for a cancelled appointment")
	}

	// Unknown and malformed IDs.
	_, err = repo.RescheduleAppointment(ctx, "00000000-0000-4000-8000-000000000000", doctor, target, allow)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown id: got %v, want ErrNotFound", err)
	}
	_, err = repo.RescheduleAppointment(ctx, "not-a-uuid", doctor, target, allow)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("malformed id: got %v, want ErrInvalidInput", err)
	}
}

// Race condition: Many different appointments try to move into the same slot at once: exactly 1 wins and the rest get ErrSlotTaken.
func TestRescheduleAppointmentConcurrent(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	doctor := newDoctor(t, pool)
	ctx := context.Background()

	start := time.Date(2031, time.June, 6, 6, 0, 0, 0, time.UTC)
	target := start.Add(5 * time.Hour)

	const attempts = 10
	ids := make([]string, attempts)
	for i := range ids {
		patient := "repo-test-resched-c" + string(rune('a'+i))
		a, err := repo.CreateAppointment(ctx, doctor, patient, start.Add(time.Duration(i)*30*time.Minute))
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
		ids[i] = a.ID
	}

	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := repo.RescheduleAppointment(ctx, id, doctor, target, allow)
			results <- err
		}(id)
	}
	wg.Wait()
	close(results)

	var won, taken int
	for err := range results {
		switch {
		case err == nil:
			won++
		case errors.Is(err, domain.ErrSlotTaken):
			taken++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if won != 1 || taken != attempts-1 {
		t.Errorf("won=%d taken=%d, want exactly 1 and %d", won, taken, attempts-1)
	}
}

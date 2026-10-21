package repository

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

func decideOnTime(domain.Appointment) (bool, error) { return false, nil }

// success path. Cancellation is allowed and slot becomes free for booking
func TestCancelAppointment(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	doctor := newDoctor(t, pool)
	ctx := context.Background()

	start := time.Date(2031, time.May, 5, 6, 0, 0, 0, time.UTC)
	booked, err := repo.CreateAppointment(ctx, doctor, "repo-test-cancel-1", start)
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}

	got, err := repo.CancelAppointment(ctx, booked.ID, "feeling unwell", func(domain.Appointment) (bool, error) { return true, nil })
	if err != nil {
		t.Fatalf("CancelAppointment: %v", err)
	}
	if got.Status != domain.StatusCancelled || !got.LateCancellation ||
		got.CancellationReason == nil || *got.CancellationReason != "feeling unwell" {
		t.Errorf("unexpected result: %+v", got)
	}
	if !got.UpdatedAt.After(booked.CreatedAt) && !got.UpdatedAt.Equal(booked.CreatedAt) {
		t.Errorf("updated_at should not go backwards: %v vs %v", got.UpdatedAt, booked.CreatedAt)
	}

	// The slot is free again: someone else can now book it.
	if _, err := repo.CreateAppointment(ctx, doctor, "repo-test-cancel-2", start); err != nil {
		t.Errorf("rebooking a cancelled slot should work: %v", err)
	}
}

// checks multiple ways cancellation fails
func TestCancelAppointmentErrors(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	doctor := newDoctor(t, pool)
	ctx := context.Background()

	start := time.Date(2031, time.May, 6, 6, 0, 0, 0, time.UTC)
	booked, err := repo.CreateAppointment(ctx, doctor, "repo-test-cancel-3", start)
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}

	// The service rule rejects the cancellation because appointment has already started.
	_, err = repo.CancelAppointment(ctx, booked.ID, "reason", func(domain.Appointment) (bool, error) {
		return false, domain.ErrAlreadyStarted
	})
	if !errors.Is(err, domain.ErrAlreadyStarted) {
		t.Errorf("got %v, want ErrAlreadyStarted", err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM appointments WHERE id = $1`, booked.ID).Scan(&status); err != nil || status != "CONFIRMED" {
		t.Errorf("a rejected cancellation must leave the appointment confirmed, got %q (%v)", status, err)
	}

	// Cancelling twice: the second attempt fails and does not call the rules
	if _, err := repo.CancelAppointment(ctx, booked.ID, "first", decideOnTime); err != nil {
		t.Fatalf("first cancel: %v", err)
	}
	called := false
	_, err = repo.CancelAppointment(ctx, booked.ID, "second", func(domain.Appointment) (bool, error) {
		called = true
		return false, nil
	})
	if !errors.Is(err, domain.ErrAlreadyCancelled) {
		t.Errorf("second cancel: got %v, want ErrAlreadyCancelled", err)
	}
	if called {
		t.Error("the rules must not run for an already-cancelled appointment")
	}

	// Unknown and malformed IDs.
	_, err = repo.CancelAppointment(ctx, "00000000-0000-4000-8000-000000000000", "reason", decideOnTime)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown id: got %v, want ErrNotFound", err)
	}
	_, err = repo.CancelAppointment(ctx, "not-a-uuid", "reason", decideOnTime)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("malformed id: got %v, want ErrInvalidInput", err)
	}
}

// Many simultaneous cancellations of the same appointment: exactly 1 wins and the rest see ErrAlreadyCancelled
func TestCancelAppointmentConcurrent(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	doctor := newDoctor(t, pool)
	ctx := context.Background()

	start := time.Date(2031, time.May, 7, 6, 0, 0, 0, time.UTC)
	booked, err := repo.CreateAppointment(ctx, doctor, "repo-test-cancel-4", start)
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}

	const attempts = 10
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.CancelAppointment(ctx, booked.ID, "concurrent", decideOnTime)
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	var cancelled, already int
	for err := range results {
		switch {
		case err == nil:
			cancelled++
		case errors.Is(err, domain.ErrAlreadyCancelled):
			already++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if cancelled != 1 || already != attempts-1 {
		t.Errorf("cancelled=%d already=%d, want exactly 1 and %d", cancelled, already, attempts-1)
	}
}

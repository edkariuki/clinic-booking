package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// check appointment creation under various scenarios: new booking, patient overlap, unknown doc etc
func TestCreateAppointment(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	doctorA := newDoctor(t, pool)
	doctorB := newDoctor(t, pool)
	ctx := context.Background()

	start := time.Date(2031, time.March, 3, 6, 0, 0, 0, time.UTC)

	// new booking is stored as CONFIRMED and lasts 30min
	got, err := repo.CreateAppointment(ctx, doctorA, "repo-test-p1", start)
	if err != nil {
		t.Fatalf("CreateAppointment: %v", err)
	}
	if got.ID == "" || got.DoctorID != doctorA || got.PatientID != "repo-test-p1" {
		t.Errorf("unexpected appointment: %+v", got)
	}
	if got.Status != domain.StatusConfirmed || got.CancellationReason != nil || got.LateCancellation {
		t.Errorf("want a confirmed appointment with no cancellation data, got %+v", got)
	}
	if !got.StartTime.Equal(start) || !got.EndTime.Equal(start.Add(30*time.Minute)) {
		t.Errorf("times = %v to %v", got.StartTime, got.EndTime)
	}

	// Same doc, same slot, different patient: the doc slot is taken.
	_, err = repo.CreateAppointment(ctx, doctorA, "repo-test-p2", start)
	if !errors.Is(err, domain.ErrSlotTaken) {
		t.Errorf("double booking: got %v, want ErrSlotTaken", err)
	}

	// Same patient, different doc, same time: the patient is already booked
	_, err = repo.CreateAppointment(ctx, doctorB, "repo-test-p1", start)
	if !errors.Is(err, domain.ErrPatientBusy) {
		t.Errorf("patient overlap: got %v, want ErrPatientBusy", err)
	}

	// Different doc and different patient at the same time is ok.
	if _, err := repo.CreateAppointment(ctx, doctorB, "repo-test-p3", start); err != nil {
		t.Errorf("independent booking should succeed: %v", err)
	}

	// doc that does not exist.
	_, err = repo.CreateAppointment(ctx, "00000000-0000-4000-8000-000000000000", "repo-test-p4", start)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown doctor: got %v, want ErrNotFound", err)
	}
}

// Race condition test: 20 attempts to book same slot at the same time
func TestCreateAppointmentConcurrentSameSlot(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	doctor := newDoctor(t, pool)
	ctx := context.Background()

	start := time.Date(2031, time.April, 7, 6, 0, 0, 0, time.UTC)
	const attempts = 20

	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := repo.CreateAppointment(ctx, doctor, fmt.Sprintf("repo-test-conc-%d", i), start)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)

	var booked, taken int
	for err := range results {
		switch {
		case err == nil:
			booked++
		case errors.Is(err, domain.ErrSlotTaken):
			taken++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if booked != 1 || taken != attempts-1 {
		t.Errorf("booked=%d taken=%d, want exactly 1 booked and %d taken", booked, taken, attempts-1)
	}
}

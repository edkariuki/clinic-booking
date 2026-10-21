package repository

import (
	"context"
	"testing"
	"time"
)

// books 3 appointmets booked out of order, response lists them correctly
func TestListUpcomingForPatient(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	doctorA := newDoctor(t, pool)
	doctorB := newDoctor(t, pool)
	ctx := context.Background()

	const patient = "repo-test-list-1"
	from := time.Date(2031, time.July, 7, 6, 0, 0, 0, time.UTC)

	book := func(doctor, who string, start time.Time) string {
		t.Helper()
		a, err := repo.CreateAppointment(ctx, doctor, who, start)
		if err != nil {
			t.Fatalf("setup booking: %v", err)
		}
		return a.ID
	}

	// Booked out of order and across two doc, to prove sorting is by time
	later := book(doctorB, patient, from.Add(48*time.Hour))
	atBoundary := book(doctorA, patient, from)
	soon := book(doctorA, patient, from.Add(2*time.Hour))

	book(doctorA, patient, from.Add(-time.Hour))
	book(doctorA, "repo-test-list-other", from.Add(time.Hour))
	cancelled := book(doctorB, patient, from.Add(24*time.Hour))
	if _, err := repo.CancelAppointment(ctx, cancelled, "test", decideOnTime); err != nil {
		t.Fatalf("setup cancel: %v", err)
	}

	got, err := repo.ListUpcomingForPatient(ctx, patient, from)
	if err != nil {
		t.Fatalf("ListUpcomingForPatient: %v", err)
	}

	want := []string{atBoundary, soon, later}
	if len(got) != len(want) {
		t.Fatalf("got %d appointments, want %d: %+v", len(got), len(want), got)
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("position %d: got %s, want %s (sorted by start time)", i, got[i].ID, id)
		}
		if got[i].PatientID != patient {
			t.Errorf("position %d belongs to %q", i, got[i].PatientID)
		}
	}
	// The upcoming list spans both doctors.
	if got[0].DoctorID != doctorA || got[2].DoctorID != doctorB {
		t.Errorf("doctors = %s, %s, want A then B", got[0].DoctorID, got[2].DoctorID)
	}
}

// checks for unknown patient that was not booked to return []
func TestListUpcomingForPatientNoResults(t *testing.T) {
	pool := testDB(t)
	repo := New(pool)
	from := time.Date(2031, time.July, 7, 6, 0, 0, 0, time.UTC)

	got, err := repo.ListUpcomingForPatient(context.Background(), "repo-test-nobody", from)
	if err != nil {
		t.Fatalf("ListUpcomingForPatient: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("got %v, want an empty non-nil slice for a patient with no appointments", got)
	}
}

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// books patient 1 with doctor 1 at earliest available slot
func TestBookSuccess(t *testing.T) {
	repo := &fakeRepo{shifts: wed}
	start := at(21, 10, 0)

	got, err := newTestService(repo).Book(context.Background(), "d1", "patient-1", start)
	if err != nil {
		t.Fatalf("Book: %v", err)
	}
	if got.Status != domain.StatusConfirmed || !got.StartTime.Equal(start) || !got.EndTime.Equal(at(21, 10, 30)) {
		t.Errorf("unexpected appointment: %+v", got)
	}
	if repo.createdFor.doctorID != "d1" || repo.createdFor.patientID != "patient-1" || !repo.createdFor.start.Equal(start) {
		t.Errorf("repository received %+v", repo.createdFor)
	}
}

// checks repo stored patient-1 with trimmed ID
func TestBookTrimsPatientID(t *testing.T) {
	repo := &fakeRepo{shifts: wed}
	if _, err := newTestService(repo).Book(context.Background(), "d1", "  patient-1 \n", at(21, 10, 0)); err != nil {
		t.Fatalf("Book: %v", err)
	}
	if repo.createdFor.patientID != "patient-1" {
		t.Errorf("patient id stored as %q, want it trimmed", repo.createdFor.patientID)
	}
}

// checks that the service rejects invalid patient IDs
func TestBookInvalidPatientID(t *testing.T) {
	tests := []struct{ name, id string }{
		{"empty", ""},
		{"only whitespace", "   "},
		{"longer than 64 characters", strings.Repeat("x", 65)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{shifts: wed}
			_, err := newTestService(repo).Book(context.Background(), "d1", tc.id, at(21, 10, 0))
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("got %v, want ErrInvalidInput", err)
			}
			if repo.createCalled {
				t.Error("nothing should be written for invalid input")
			}
		})
	}
}

// checks that the service rejects unknown doc
func TestBookUnknownDoctor(t *testing.T) {
	repo := &fakeRepo{doctorErr: domain.ErrNotFound, shifts: wed}
	_, err := newTestService(repo).Book(context.Background(), "d1", "patient-1", at(21, 10, 0))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
	if repo.createCalled {
		t.Error("nothing should be written for an unknown doctor")
	}
}

// checks that the service rejects slots that are invalid according to the rules eg past time, within 1hr buffer...
func TestBookRejectsInvalidSlots(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
		want  error
	}{
		{"in the past", at(21, 8, 0), domain.ErrInPast},
		{"inside the 1-hour buffer", at(21, 9, 30), domain.ErrTooSoon},
		{"outside working hours", at(21, 12, 0), domain.ErrOutsideWorkingHours},
		{"not on the 30-minute grid", at(21, 10, 15), domain.ErrMisaligned},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{shifts: wed}
			_, err := newTestService(repo).Book(context.Background(), "d1", "patient-1", tc.start)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
			if repo.createCalled {
				t.Error("an invalid slot must never reach the database")
			}
		})
	}
}

// checks that the service passes thru the repo conflict errors
func TestBookPassesThroughConflicts(t *testing.T) {
	for _, conflict := range []error{domain.ErrSlotTaken, domain.ErrPatientBusy} {
		repo := &fakeRepo{shifts: wed, createErr: conflict}
		_, err := newTestService(repo).Book(context.Background(), "d1", "patient-1", at(21, 10, 0))
		if !errors.Is(err, conflict) {
			t.Errorf("got %v, want %v", err, conflict)
		}
	}
}

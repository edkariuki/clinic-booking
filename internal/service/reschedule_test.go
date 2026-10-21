package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// now is Wednesday 09:00. Appointment at 12:00 is far enough away to be moved.
func movable() domain.Appointment { return confirmedAt(at(21, 12, 0)) }

// reschedule same doc
func TestRescheduleSameDoctor(t *testing.T) {
	repo := &fakeRepo{shifts: wed, getAppt: movable()}
	target := at(21, 14, 0)

	got, err := newTestService(repo).Reschedule(context.Background(), "appt-1", "", target)
	if err != nil {
		t.Fatalf("Reschedule: %v", err)
	}
	if got.DoctorID != "d1" || !got.StartTime.Equal(target) || !got.EndTime.Equal(at(21, 14, 30)) {
		t.Errorf("unexpected appointment: %+v", got)
	}
	if repo.rescheduled.id != "appt-1" || repo.rescheduled.doctorID != "d1" || !repo.rescheduled.start.Equal(target) {
		t.Errorf("repository received %+v", repo.rescheduled)
	}
	if len(repo.doctorLookups) != 0 {
		t.Errorf("the current doctor needs no lookup, but looked up %v", repo.doctorLookups)
	}
	if repo.shiftsFor != "d1" {
		t.Errorf("shifts loaded for %q, want d1", repo.shiftsFor)
	}
}

// reschedule different doc
func TestRescheduleToAnotherDoctor(t *testing.T) {
	repo := &fakeRepo{shifts: wed, getAppt: movable()}
	target := at(21, 14, 0)

	got, err := newTestService(repo).Reschedule(context.Background(), "appt-1", "  d2 ", target)
	if err != nil {
		t.Fatalf("Reschedule: %v", err)
	}
	if got.DoctorID != "d2" || repo.rescheduled.doctorID != "d2" {
		t.Errorf("want the appointment moved to d2, got %+v (repo %+v)", got, repo.rescheduled)
	}
	if len(repo.doctorLookups) != 1 || repo.doctorLookups[0] != "d2" {
		t.Errorf("doctor lookups = %v, want just d2 (id trimmed)", repo.doctorLookups)
	}
	if repo.shiftsFor != "d2" {
		t.Errorf("the new slot must be checked against d2's shifts, loaded %q", repo.shiftsFor)
	}
}

// checks unknown doc expecting no writes
func TestRescheduleUnknownAppointmentOrDoctor(t *testing.T) {
	t.Run("unknown appointment", func(t *testing.T) {
		repo := &fakeRepo{shifts: wed, getErr: domain.ErrNotFound}
		_, err := newTestService(repo).Reschedule(context.Background(), "nope", "", at(21, 14, 0))
		if !errors.Is(err, domain.ErrNotFound) || repo.rescheduleCalled {
			t.Errorf("got %v (reschedule called: %v), want ErrNotFound and no write", err, repo.rescheduleCalled)
		}
	})
	t.Run("malformed appointment id", func(t *testing.T) {
		repo := &fakeRepo{shifts: wed, getErr: domain.ErrInvalidInput}
		_, err := newTestService(repo).Reschedule(context.Background(), "x", "", at(21, 14, 0))
		if !errors.Is(err, domain.ErrInvalidInput) || repo.rescheduleCalled {
			t.Errorf("got %v, want ErrInvalidInput and no write", err)
		}
	})
	t.Run("unknown target doctor", func(t *testing.T) {
		repo := &fakeRepo{shifts: wed, getAppt: movable(), doctorErr: domain.ErrNotFound}
		_, err := newTestService(repo).Reschedule(context.Background(), "appt-1", "d9", at(21, 14, 0))
		if !errors.Is(err, domain.ErrNotFound) || repo.rescheduleCalled {
			t.Errorf("got %v, want ErrNotFound and no write", err)
		}
	})
}

// checks multiple rejected reschedules each with its own reason
func TestRescheduleRejectsInvalidTargetSlots(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
		want  error
	}{
		{"in the past", at(21, 8, 0), domain.ErrInPast},
		{"inside the 1-hour buffer", at(21, 9, 30), domain.ErrTooSoon},
		{"outside working hours", at(21, 12, 30), domain.ErrOutsideWorkingHours},
		{"not on the 30-minute grid", at(21, 14, 15), domain.ErrMisaligned},
		{"a day the doctor does not work", at(22, 10, 0), domain.ErrOutsideWorkingHours},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{shifts: wed, getAppt: movable()}
			_, err := newTestService(repo).Reschedule(context.Background(), "appt-1", "", tc.start)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
			if repo.rescheduleApplied {
				t.Error("an invalid target slot must never be committed")
			}
		})
	}
}

// checks the rules about reschedules eg 1hr buffer
func TestRescheduleChecksTheCurrentAppointment(t *testing.T) {
	tests := []struct {
		name    string
		current time.Time
		want    error
	}{
		{"exactly 1 hour away can still be moved", at(21, 10, 0), nil},
		{"59 minutes away is too late", at(21, 9, 59), domain.ErrRescheduleTooLate},
		{"30 minutes away is too late", at(21, 9, 30), domain.ErrRescheduleTooLate},
		{"already started", at(21, 8, 30), domain.ErrAlreadyStarted},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{shifts: wed, getAppt: confirmedAt(tc.current)}
			_, err := newTestService(repo).Reschedule(context.Background(), "appt-1", "", at(21, 14, 0))
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
			if applied := repo.rescheduleApplied; applied != (tc.want == nil) {
				t.Errorf("applied = %v, want %v", applied, tc.want == nil)
			}
		})
	}
}

// checks cancelled appointment cannot be moved
func TestRescheduleCancelledAppointment(t *testing.T) {
	cancelled := movable()
	cancelled.Status = domain.StatusCancelled
	repo := &fakeRepo{shifts: wed, getAppt: cancelled}

	_, err := newTestService(repo).Reschedule(context.Background(), "appt-1", "", at(21, 14, 0))
	if !errors.Is(err, domain.ErrAlreadyCancelled) || repo.rescheduleApplied {
		t.Errorf("got %v (applied %v), want ErrAlreadyCancelled and no move", err, repo.rescheduleApplied)
	}
}

// checks that the service passes thru the conflict errors
func TestReschedulePassesThroughConflicts(t *testing.T) {
	for _, conflict := range []error{domain.ErrSlotTaken, domain.ErrPatientBusy} {
		repo := &fakeRepo{shifts: wed, getAppt: movable(), rescheduleErr: conflict}
		_, err := newTestService(repo).Reschedule(context.Background(), "appt-1", "", at(21, 14, 0))
		if !errors.Is(err, conflict) {
			t.Errorf("got %v, want %v", err, conflict)
		}
	}
}

package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

func confirmedAt(start time.Time) domain.Appointment {
	return domain.Appointment{
		ID: "appt-1", DoctorID: "d1", PatientID: "patient-1",
		StartTime: start, EndTime: start.Add(domain.SlotDuration), Status: domain.StatusConfirmed,
	}
}

// cancels appointment with plenty of notice time and with reason
func TestCancelWithPlentyOfNotice(t *testing.T) {
	repo := &fakeRepo{cancelAppt: confirmedAt(at(21, 12, 0))}
	got, err := newTestService(repo).Cancel(context.Background(), "appt-1", "  feeling unwell \n")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if got.Status != domain.StatusCancelled || got.LateCancellation {
		t.Errorf("want a normal cancellation, got %+v", got)
	}
	if repo.cancelledID != "appt-1" || repo.cancelReason != "feeling unwell" {
		t.Errorf("repository received id=%q reason=%q, want the reason trimmed", repo.cancelledID, repo.cancelReason)
	}
	if got.CancellationReason == nil || *got.CancellationReason != "feeling unwell" {
		t.Errorf("reason = %v", got.CancellationReason)
	}
}

// cancels appointment with less than 1hr notice and with reason. The cancellation is flagged
func TestCancelLateFlag(t *testing.T) {
	tests := []struct {
		name     string
		start    time.Time
		wantLate bool
	}{
		{"exactly 1 hour ahead is not late", at(21, 10, 0), false},
		{"59 minutes ahead is late", at(21, 9, 59), true},
		{"45 minutes ahead is late", at(21, 9, 45), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{cancelAppt: confirmedAt(tc.start)}
			got, err := newTestService(repo).Cancel(context.Background(), "appt-1", "reason")
			if err != nil {
				t.Fatalf("Cancel: %v", err)
			}
			if got.LateCancellation != tc.wantLate {
				t.Errorf("late = %v, want %v", got.LateCancellation, tc.wantLate)
			}
		})
	}
}

// tries cancel an appointment that has already started
func TestCancelAlreadyStarted(t *testing.T) {
	for _, start := range []time.Time{at(21, 9, 0), at(21, 8, 30)} {
		repo := &fakeRepo{cancelAppt: confirmedAt(start)}
		_, err := newTestService(repo).Cancel(context.Background(), "appt-1", "reason")
		if !errors.Is(err, domain.ErrAlreadyStarted) {
			t.Errorf("start %v: got %v, want ErrAlreadyStarted", start, err)
		}
	}
}

// checks reason is validated
func TestCancelInvalidReason(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   error
	}{
		{"empty", "", domain.ErrReasonRequired},
		{"only whitespace", "  \t ", domain.ErrReasonRequired},
		{"501 characters", strings.Repeat("a", 501), domain.ErrReasonTooLong},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{cancelAppt: confirmedAt(at(21, 12, 0))}
			_, err := newTestService(repo).Cancel(context.Background(), "appt-1", tc.reason)
			if !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
			if repo.cancelCalled {
				t.Error("an invalid reason must be rejected before touching the database")
			}
		})
	}
}

// checks that repository errors are passed thru
func TestCancelPassesThroughRepositoryErrors(t *testing.T) {
	for _, repoErr := range []error{domain.ErrNotFound, domain.ErrAlreadyCancelled, domain.ErrInvalidInput} {
		repo := &fakeRepo{cancelErr: repoErr}
		_, err := newTestService(repo).Cancel(context.Background(), "appt-1", "reason")
		if !errors.Is(err, repoErr) {
			t.Errorf("got %v, want %v", err, repoErr)
		}
	}
}

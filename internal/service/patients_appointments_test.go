package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// successfully lists well sorted upcoming appointments
func TestUpcomingForPatient(t *testing.T) {
	list := []domain.Appointment{confirmedAt(at(21, 12, 0)), confirmedAt(at(22, 10, 0))}
	repo := &fakeRepo{upcoming: list}

	got, err := newTestService(repo).UpcomingForPatient(context.Background(), "  patient-1 \n")
	if err != nil {
		t.Fatalf("UpcomingForPatient: %v", err)
	}
	if len(got) != 2 || got[0].ID != list[0].ID {
		t.Errorf("got %+v, want the repository's list unchanged", got)
	}
	if repo.upcomingCall.patientID != "patient-1" {
		t.Errorf("repository asked for %q, want the id trimmed", repo.upcomingCall.patientID)
	}
	if !repo.upcomingCall.from.Equal(now) {
		t.Errorf("upcoming measured from %v, want the service clock %v", repo.upcomingCall.from, now)
	}
}

// returns [] for no upcoming appointment list
func TestUpcomingForPatientNone(t *testing.T) {
	repo := &fakeRepo{upcoming: []domain.Appointment{}}
	got, err := newTestService(repo).UpcomingForPatient(context.Background(), "nobody")
	if err != nil || got == nil || len(got) != 0 {
		t.Errorf("got (%v, %v), want an empty non-nil list and no error", got, err)
	}
}

// checks upcoming appointments with bad IDs
func TestUpcomingForPatientInvalidID(t *testing.T) {
	tests := []struct{ name, id string }{
		{"empty", ""},
		{"only whitespace", "   "},
		{"longer than 64 characters", strings.Repeat("x", 65)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{}
			_, err := newTestService(repo).UpcomingForPatient(context.Background(), tc.id)
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("got %v, want ErrInvalidInput", err)
			}
			if repo.upcomingCall.called {
				t.Error("the database must not be queried for an invalid id")
			}
		})
	}
}

// checks generic error is returned
func TestUpcomingForPatientRepositoryError(t *testing.T) {
	boom := errors.New("db down")
	_, err := newTestService(&fakeRepo{upcomingErr: boom}).UpcomingForPatient(context.Background(), "patient-1")
	if !errors.Is(err, boom) {
		t.Errorf("got %v, want the repository error passed through", err)
	}
}

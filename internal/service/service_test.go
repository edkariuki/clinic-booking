package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

type fakeRepo struct {
	doctorErr error
	shifts    []domain.Shift
	taken     []time.Time

	gotFrom, gotTo time.Time
	shiftsCalled   bool

	createErr    error
	createCalled bool
	createdFor   struct {
		doctorID, patientID string
		start               time.Time
	}

	cancelAppt   domain.Appointment
	cancelErr    error
	cancelCalled bool
	cancelledID  string
	cancelReason string

	getAppt           domain.Appointment
	getErr            error
	rescheduleErr     error
	rescheduleCalled  bool
	rescheduleApplied bool
	rescheduled       struct {
		id, doctorID string
		start        time.Time
	}
	shiftsFor     string
	doctorLookups []string
}

func (f *fakeRepo) GetDoctor(_ context.Context, id string) (domain.Doctor, error) {
	if f.doctorErr != nil {
		return domain.Doctor{}, f.doctorErr
	}
	return domain.Doctor{ID: id, Name: "Dr. Test"}, nil
}

func (f *fakeRepo) ListShifts(_ context.Context, _ string) ([]domain.Shift, error) {
	f.shiftsCalled = true
	return f.shifts, nil
}

func (f *fakeRepo) ListConfirmedStarts(_ context.Context, _ string, from, to time.Time) ([]time.Time, error) {
	f.gotFrom, f.gotTo = from, to
	return f.taken, nil
}

func (f *fakeRepo) CreateAppointment(_ context.Context, doctorID, patientID string, start time.Time) (domain.Appointment, error) {
	f.createCalled = true
	f.createdFor.doctorID, f.createdFor.patientID, f.createdFor.start = doctorID, patientID, start
	if f.createErr != nil {
		return domain.Appointment{}, f.createErr
	}
	return domain.Appointment{
		ID: "appt-1", DoctorID: doctorID, PatientID: patientID,
		StartTime: start, EndTime: start.Add(domain.SlotDuration), Status: domain.StatusConfirmed,
	}, nil
}

func (f *fakeRepo) CancelAppointment(_ context.Context, id, reason string, decide func(domain.Appointment) (bool, error)) (domain.Appointment, error) {
	f.cancelCalled = true
	f.cancelledID, f.cancelReason = id, reason
	if f.cancelErr != nil {
		return domain.Appointment{}, f.cancelErr
	}
	late, err := decide(f.cancelAppt)
	if err != nil {
		return domain.Appointment{}, err
	}
	out := f.cancelAppt
	out.Status = domain.StatusCancelled
	out.CancellationReason = &reason
	out.LateCancellation = late
	return out, nil
}

func (f *fakeRepo) GetAppointment(_ context.Context, _ string) (domain.Appointment, error) {
	if f.getErr != nil {
		return domain.Appointment{}, f.getErr
	}
	return f.getAppt, nil
}

func (f *fakeRepo) RescheduleAppointment(_ context.Context, id, doctorID string, start time.Time, decide func(domain.Appointment) error) (domain.Appointment, error) {
	f.rescheduleCalled = true
	f.rescheduled.id, f.rescheduled.doctorID, f.rescheduled.start = id, doctorID, start
	if f.rescheduleErr != nil {
		return domain.Appointment{}, f.rescheduleErr
	}
	if f.getAppt.Status == domain.StatusCancelled {
		return domain.Appointment{}, domain.ErrAlreadyCancelled
	}
	if err := decide(f.getAppt); err != nil {
		return domain.Appointment{}, err
	}
	f.rescheduleApplied = true
	out := f.getAppt
	out.DoctorID = doctorID
	out.StartTime = start
	out.EndTime = start.Add(domain.SlotDuration)
	return out, nil
}

func newTestService(repo *fakeRepo) *Service {
	return New(repo, func() time.Time { return now })
}

// no shifts for an unkonwn doc
func TestAvailabilityUnknownDoctor(t *testing.T) {
	repo := &fakeRepo{doctorErr: domain.ErrNotFound}
	_, err := newTestService(repo).Availability(context.Background(), "d1", at(21, 0, 0))
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
	if repo.shiftsCalled {
		t.Error("shifts should not be loaded for an unknown doctor")
	}
}

// checks availbale slots removing the taken ones and checking the 1hr buffer
func TestAvailabilityExcludesTakenAndBufferedSlots(t *testing.T) {
	repo := &fakeRepo{shifts: wed, taken: []time.Time{at(21, 10, 30), at(21, 13, 0)}}

	got, err := newTestService(repo).Availability(context.Background(), "d1", at(21, 15, 0))
	if err != nil {
		t.Fatalf("Availability: %v", err)
	}
	if len(got) != 10 || !got[0].Equal(at(21, 10, 0)) {
		t.Errorf("got %d slots starting %v, want 10 starting at 10:00", len(got), got)
	}
}

// checks the day according to clinic timezone
func TestAvailabilityAsksForTheClinicDay(t *testing.T) {
	repo := &fakeRepo{shifts: wed}

	tue2200UTC := time.Date(2026, time.October, 20, 22, 0, 0, 0, time.UTC)
	if _, err := newTestService(repo).Availability(context.Background(), "d1", tue2200UTC); err != nil {
		t.Fatalf("Availability: %v", err)
	}
	if !repo.gotFrom.Equal(at(21, 0, 0)) || !repo.gotTo.Equal(at(22, 0, 0)) {
		t.Errorf("queried [%v, %v), want Wednesday midnight to Thursday midnight (Nairobi)", repo.gotFrom, repo.gotTo)
	}
}

// checks nil is returned for past date
func TestAvailabilityPastDateIsEmpty(t *testing.T) {
	repo := &fakeRepo{shifts: wed}
	got, err := newTestService(repo).Availability(context.Background(), "d1", at(14, 0, 0))
	if err != nil {
		t.Fatalf("Availability: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("got %v, want an empty non-nil slice", got)
	}
}

// check nil is returned for day the doc is not working
func TestAvailabilityDayWithoutShifts(t *testing.T) {
	repo := &fakeRepo{shifts: wed}
	got, err := newTestService(repo).Availability(context.Background(), "d1", at(22, 0, 0))
	if err != nil || len(got) != 0 {
		t.Errorf("got (%v, %v), want empty and no error", got, err)
	}
}

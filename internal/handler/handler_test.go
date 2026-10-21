package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

const doctorID = "11111111-1111-4111-8111-111111111111"

type fakeService struct {
	slots []time.Time
	err   error

	gotDoctor string
	gotDay    time.Time

	appt       domain.Appointment
	bookErr    error
	bookCalled bool
	gotBook    struct {
		doctorID, patientID string
		start               time.Time
	}

	cancelAppt   domain.Appointment
	cancelErr    error
	cancelCalled bool
	gotCancel    struct{ id, reason string }
}

func (f *fakeService) Availability(_ context.Context, id string, day time.Time) ([]time.Time, error) {
	f.gotDoctor, f.gotDay = id, day
	return f.slots, f.err
}

func (f *fakeService) Book(_ context.Context, doctorID, patientID string, start time.Time) (domain.Appointment, error) {
	f.bookCalled = true
	f.gotBook.doctorID, f.gotBook.patientID, f.gotBook.start = doctorID, patientID, start
	return f.appt, f.bookErr
}

func (f *fakeService) Cancel(_ context.Context, id, reason string) (domain.Appointment, error) {
	f.cancelCalled = true
	f.gotCancel.id, f.gotCancel.reason = id, reason
	return f.cancelAppt, f.cancelErr
}

func do(t *testing.T, svc Service, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	New(svc).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

// checks successful response with slots
func TestAvailabilitySuccess(t *testing.T) {
	loc := domain.ClinicLocation
	svc := &fakeService{slots: []time.Time{
		time.Date(2026, 10, 22, 8, 0, 0, 0, loc),
		time.Date(2026, 10, 22, 8, 30, 0, 0, loc),
	}}

	rec := do(t, svc, fmt.Sprintf("/doctors/%s/availability?date=2026-10-22", doctorID))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var got availabilityResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.DoctorID != doctorID || got.Date != "2026-10-22" || len(got.Slots) != 2 {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(rec.Body.String(), `"start":"2026-10-22T08:00:00+03:00"`) ||
		!strings.Contains(rec.Body.String(), `"end":"2026-10-22T08:30:00+03:00"`) {
		t.Errorf("times should be RFC 3339 in Nairobi time, body: %s", rec.Body)
	}
	if svc.gotDoctor != doctorID || svc.gotDay.Format("2006-01-02") != "2026-10-22" {
		t.Errorf("service called with %q, %v", svc.gotDoctor, svc.gotDay)
	}
}

// checks [] is returned for no slots
func TestAvailabilityEmptyEncodesAsArray(t *testing.T) {
	rec := do(t, &fakeService{slots: nil}, fmt.Sprintf("/doctors/%s/availability?date=2026-10-22", doctorID))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"slots":[]`) {
		t.Errorf("status %d, body %s; want 200 with an empty array, not null", rec.Code, rec.Body)
	}
}

// checks bad req with missing date, wrong format... checking also correct error msg in JSON body
func TestAvailabilityBadRequests(t *testing.T) {
	tests := []struct {
		name, target string
	}{
		{"missing date", fmt.Sprintf("/doctors/%s/availability", doctorID)},
		{"date in wrong format", fmt.Sprintf("/doctors/%s/availability?date=22-10-2026", doctorID)},
		{"impossible date", fmt.Sprintf("/doctors/%s/availability?date=2026-13-45", doctorID)},
		{"doctor id is not a UUID", "/doctors/abc/availability?date=2026-10-22"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{}
			rec := do(t, svc, tc.target)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body %s", rec.Code, rec.Body)
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error == "" {
				t.Errorf("want a JSON error message, got %s", rec.Body)
			}
			if svc.gotDoctor != "" {
				t.Error("service must not be called for invalid input")
			}
		})
	}
}

// checks unkon doctor availability returning 404
func TestAvailabilityUnknownDoctor(t *testing.T) {
	rec := do(t, &fakeService{err: domain.ErrNotFound}, fmt.Sprintf("/doctors/%s/availability?date=2026-10-22", doctorID))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// checks service error is returned as 500
func TestUnexpectedErrorDoesNotLeak(t *testing.T) {
	rec := do(t, &fakeService{err: errors.New("pq: connection to 10.0.0.5 refused")},
		fmt.Sprintf("/doctors/%s/availability?date=2026-10-22", doctorID))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Errorf("internal details leaked to the client: %s", rec.Body)
	}
}

// checks each domain err agains the correct HTTP status code
func TestStatusFor(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{domain.ErrInvalidInput, 400},
		{domain.ErrReasonRequired, 400},
		{domain.ErrReasonTooLong, 400},
		{domain.ErrNotFound, 404},
		{domain.ErrSlotTaken, 409},
		{domain.ErrPatientBusy, 409},
		{domain.ErrAlreadyCancelled, 409},
		{domain.ErrInPast, 422},
		{domain.ErrTooSoon, 422},
		{domain.ErrOutsideWorkingHours, 422},
		{domain.ErrMisaligned, 422},
		{domain.ErrAlreadyStarted, 422},
		{domain.ErrRescheduleTooLate, 422},
		{fmt.Errorf("%w: wrapped", domain.ErrNotFound), 404},
		{errors.New("something else"), 500},
	}
	for _, tc := range tests {
		if got := statusFor(tc.err); got != tc.want {
			t.Errorf("statusFor(%v) = %d, want %d", tc.err, got, tc.want)
		}
	}
}

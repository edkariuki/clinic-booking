package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

// successfully returns list of 2 confirmed appointments
func TestPatientAppointmentsSuccess(t *testing.T) {
	loc := domain.ClinicLocation
	first := time.Date(2026, 10, 27, 10, 0, 0, 0, loc)
	second := time.Date(2026, 10, 28, 14, 30, 0, 0, loc)
	svc := &fakeService{upcoming: []domain.Appointment{
		{ID: "appt-1", DoctorID: doctorID, PatientID: "patient-1", StartTime: first, EndTime: first.Add(30 * time.Minute), Status: domain.StatusConfirmed},
		{ID: "appt-2", DoctorID: doctorID, PatientID: "patient-1", StartTime: second, EndTime: second.Add(30 * time.Minute), Status: domain.StatusConfirmed},
	}}

	rec := do(t, svc, "/patients/patient-1/appointments")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var got patientAppointmentsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.PatientID != "patient-1" || len(got.Appointments) != 2 {
		t.Fatalf("unexpected response %+v", got)
	}
	if got.Appointments[0].ID != "appt-1" || !got.Appointments[0].StartTime.Equal(first) ||
		got.Appointments[1].ID != "appt-2" || got.Appointments[1].Status != "CONFIRMED" {
		t.Errorf("appointments out of order or wrong: %+v", got.Appointments)
	}
	if svc.gotPatient != "patient-1" {
		t.Errorf("service received %q", svc.gotPatient)
	}
	if !strings.Contains(rec.Body.String(), "+03:00") {
		t.Errorf("times should be in clinic time (+03:00): %s", rec.Body)
	}
}

// checks that [] is returned and not nil
func TestPatientAppointmentsEmptyIsAnEmptyList(t *testing.T) {
	rec := do(t, &fakeService{}, "/patients/nobody/appointments")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"appointments":[]`) {
		t.Errorf("want an empty array, got %s", rec.Body)
	}
}

// checks that handler does not reject or alter IDs
func TestPatientAppointmentsPassesTheIDThrough(t *testing.T) {
	svc := &fakeService{}
	rec := do(t, svc, "/patients/user_42%40example.com/appointments")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if svc.gotPatient != "user_42@example.com" {
		t.Errorf("service received %q, want the decoded path value", svc.gotPatient)
	}
}

// checks that service errors are mapped to the correct HTTP status codes
func TestPatientAppointmentsMapsServiceErrors(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{domain.ErrInvalidInput, http.StatusBadRequest},
		{http.ErrAbortHandler, http.StatusInternalServerError},
	}
	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			rec := do(t, &fakeService{upcomingErr: tc.err}, "/patients/patient-1/appointments")
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error == "" {
				t.Errorf("want a JSON error message, got %s", rec.Body)
			}
			if tc.want == http.StatusInternalServerError && strings.Contains(rec.Body.String(), tc.err.Error()) {
				t.Errorf("internal errors must not leak to clients: %s", rec.Body)
			}
		})
	}
}

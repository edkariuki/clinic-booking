package handler

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

const (
	rescheduleURL  = "/appointments/" + appointmentID + "/reschedule"
	otherDoctorID  = "22222222-2222-4222-8222-222222222222"
	newStartString = "2026-10-27T14:00:00+03:00"
)

func movedAppointment(doctor string, start time.Time) domain.Appointment {
	return domain.Appointment{
		ID: appointmentID, DoctorID: doctor, PatientID: "patient-1",
		StartTime: start, EndTime: start.Add(30 * time.Minute), Status: domain.StatusConfirmed,
	}
}

// checks reschedule of same doc with enough time for reschedule
func TestRescheduleSameDoctor(t *testing.T) {
	start := time.Date(2026, 10, 27, 14, 0, 0, 0, domain.ClinicLocation)
	svc := &fakeService{rescheduleAppt: movedAppointment(doctorID, start)}

	rec := patch(t, svc, rescheduleURL, `{"start_time":"`+newStartString+`"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var got appointmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != appointmentID || got.Status != "CONFIRMED" || !got.StartTime.Equal(start) || !got.EndTime.Equal(start.Add(30*time.Minute)) {
		t.Errorf("unexpected response %+v", got)
	}
	g := svc.gotReschedule
	if g.id != appointmentID || g.doctorID != "" || !g.start.Equal(start) {
		t.Errorf("service received %+v; with no doctor_id it should get an empty doctor", g)
	}
}

// checks reschedule of different doc with enough time for reschedule
func TestRescheduleToAnotherDoctor(t *testing.T) {
	start := time.Date(2026, 10, 27, 14, 0, 0, 0, domain.ClinicLocation)
	svc := &fakeService{rescheduleAppt: movedAppointment(otherDoctorID, start)}

	rec := patch(t, svc, rescheduleURL, `{"start_time":"`+newStartString+`","doctor_id":"`+otherDoctorID+`"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var got appointmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || got.DoctorID != otherDoctorID {
		t.Errorf("response doctor = %q (err %v), want %s", got.DoctorID, err, otherDoctorID)
	}
	if svc.gotReschedule.doctorID != otherDoctorID {
		t.Errorf("service received doctor %q, want %s", svc.gotReschedule.doctorID, otherDoctorID)
	}
}

// checks bad req with correct responses
func TestRescheduleBadRequests(t *testing.T) {
	tests := []struct{ name, target, body string }{
		{"appointment id is not a UUID", "/appointments/abc/reschedule", `{"start_time":"` + newStartString + `"}`},
		{"empty body", rescheduleURL, ``},
		{"not JSON", rescheduleURL, `tomorrow at 2`},
		{"missing start_time", rescheduleURL, `{}`},
		{"start_time without an offset", rescheduleURL, `{"start_time":"2026-10-27T14:00:00"}`},
		{"start_time is not a timestamp", rescheduleURL, `{"start_time":"tomorrow"}`},
		{"start_time has the wrong type", rescheduleURL, `{"start_time":1730000000}`},
		{"doctor_id is not a UUID", rescheduleURL, `{"start_time":"` + newStartString + `","doctor_id":"dr-bob"}`},
		{"doctor_id is empty", rescheduleURL, `{"start_time":"` + newStartString + `","doctor_id":""}`},
		{"unknown field", rescheduleURL, `{"start_time":"` + newStartString + `","patient_id":"someone-else"}`},
		{"trailing data", rescheduleURL, `{"start_time":"` + newStartString + `"}{}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{}
			rec := patch(t, svc, tc.target, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body %s", rec.Code, rec.Body)
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error == "" {
				t.Errorf("want a JSON error message, got %s", rec.Body)
			}
			if svc.rescheduleCalled {
				t.Error("the service must not be called for an invalid request")
			}
		})
	}
}

// checks that service errors are mapped to the correct HTTP status codes
func TestRescheduleMapsServiceErrors(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{domain.ErrInvalidInput, http.StatusBadRequest},
		{domain.ErrNotFound, http.StatusNotFound},
		{domain.ErrSlotTaken, http.StatusConflict},
		{domain.ErrPatientBusy, http.StatusConflict},
		{domain.ErrAlreadyCancelled, http.StatusConflict},
		{domain.ErrInPast, http.StatusUnprocessableEntity},
		{domain.ErrTooSoon, http.StatusUnprocessableEntity},
		{domain.ErrOutsideWorkingHours, http.StatusUnprocessableEntity},
		{domain.ErrMisaligned, http.StatusUnprocessableEntity},
		{domain.ErrAlreadyStarted, http.StatusUnprocessableEntity},
		{domain.ErrRescheduleTooLate, http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			rec := patch(t, &fakeService{rescheduleErr: tc.err}, rescheduleURL, `{"start_time":"`+newStartString+`"}`)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error == "" {
				t.Errorf("want a JSON error message, got %s", rec.Body)
			}
		})
	}
}

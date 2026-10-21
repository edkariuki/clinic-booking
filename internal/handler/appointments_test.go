package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

func post(t *testing.T, svc Service, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	New(svc).Register(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/appointments", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	return rec
}

const validBody = `{"doctor_id":"11111111-1111-4111-8111-111111111111","patient_id":"patient-1","start_time":"2026-10-26T10:00:00+03:00"}`

// creates successful appointment and return correct response
func TestCreateAppointmentSuccess(t *testing.T) {
	loc := domain.ClinicLocation
	start := time.Date(2026, 10, 26, 10, 0, 0, 0, loc)
	svc := &fakeService{appt: domain.Appointment{
		ID: "appt-1", DoctorID: doctorID, PatientID: "patient-1",
		StartTime: start, EndTime: start.Add(30 * time.Minute), Status: domain.StatusConfirmed,
	}}

	rec := post(t, svc, validBody)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var got appointmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != "appt-1" || got.Status != "CONFIRMED" || got.LateCancellation || got.CancellationReason != nil {
		t.Errorf("unexpected response %+v", got)
	}
	if !strings.Contains(rec.Body.String(), `"start_time":"2026-10-26T10:00:00+03:00"`) ||
		!strings.Contains(rec.Body.String(), `"end_time":"2026-10-26T10:30:00+03:00"`) ||
		!strings.Contains(rec.Body.String(), `"cancellation_reason":null`) {
		t.Errorf("unexpected body: %s", rec.Body)
	}
	if svc.gotBook.doctorID != doctorID || svc.gotBook.patientID != "patient-1" || !svc.gotBook.start.Equal(start) {
		t.Errorf("service received %+v", svc.gotBook)
	}
}

// checks can send UTC timestamp and service receives the same instant as Nairobi time
func TestCreateAppointmentAcceptsUTCTimestamp(t *testing.T) {
	svc := &fakeService{}
	body := `{"doctor_id":"` + doctorID + `","patient_id":"p","start_time":"2026-10-26T07:00:00Z"}`
	post(t, svc, body)
	if want := time.Date(2026, 10, 26, 7, 0, 0, 0, time.UTC); !svc.gotBook.start.Equal(want) {
		t.Errorf("start = %v, want the same instant as 10:00 Nairobi (%v)", svc.gotBook.start, want)
	}
}

// reject bad req bodies and never calls the service
func TestCreateAppointmentBadRequests(t *testing.T) {
	tests := []struct{ name, body string }{
		{"empty body", ``},
		{"not JSON", `not json`},
		{"trailing data", validBody + `{}`},
		{"unknown field", `{"doctor_id":"` + doctorID + `","patient_id":"p","start_time":"2026-10-26T10:00:00+03:00","status":"CANCELLED"}`},
		{"wrong type", `{"doctor_id":"` + doctorID + `","patient_id":42,"start_time":"2026-10-26T10:00:00+03:00"}`},
		{"missing doctor_id", `{"patient_id":"p","start_time":"2026-10-26T10:00:00+03:00"}`},
		{"doctor_id not a UUID", `{"doctor_id":"abc","patient_id":"p","start_time":"2026-10-26T10:00:00+03:00"}`},
		{"missing start_time", `{"doctor_id":"` + doctorID + `","patient_id":"p"}`},
		{"start_time without offset", `{"doctor_id":"` + doctorID + `","patient_id":"p","start_time":"2026-10-26T10:00:00"}`},
		{"start_time not a timestamp", `{"doctor_id":"` + doctorID + `","patient_id":"p","start_time":"tomorrow at ten"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{}
			rec := post(t, svc, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body %s", rec.Code, rec.Body)
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error == "" {
				t.Errorf("want a JSON error message, got %s", rec.Body)
			}
			if svc.bookCalled {
				t.Error("the service must not be called for an invalid request")
			}
		})
	}
}

// checks that service errors are mapped to the correct HTTP status codes
func TestCreateAppointmentMapsServiceErrors(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{domain.ErrInvalidInput, http.StatusBadRequest},
		{domain.ErrNotFound, http.StatusNotFound},
		{domain.ErrSlotTaken, http.StatusConflict},
		{domain.ErrPatientBusy, http.StatusConflict},
		{domain.ErrInPast, http.StatusUnprocessableEntity},
		{domain.ErrTooSoon, http.StatusUnprocessableEntity},
		{domain.ErrOutsideWorkingHours, http.StatusUnprocessableEntity},
		{domain.ErrMisaligned, http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			rec := post(t, &fakeService{bookErr: tc.err}, validBody)
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

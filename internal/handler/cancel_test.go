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

const appointmentID = "42e3d991-1dd5-4e36-916a-136b0f68f42c"

func patch(t *testing.T, svc Service, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	New(svc).Register(mux)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	return rec
}

// successful cancel appointment test with reason and late cancellation flag
func TestCancelAppointmentSuccess(t *testing.T) {
	start := time.Date(2026, 10, 26, 10, 0, 0, 0, domain.ClinicLocation)
	reason := "feeling unwell"
	svc := &fakeService{cancelAppt: domain.Appointment{
		ID: appointmentID, DoctorID: doctorID, PatientID: "patient-1",
		StartTime: start, EndTime: start.Add(30 * time.Minute),
		Status: domain.StatusCancelled, CancellationReason: &reason, LateCancellation: true,
	}}

	rec := patch(t, svc, "/appointments/"+appointmentID+"/cancel", `{"reason":"feeling unwell"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var got appointmentResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != appointmentID || got.Status != "CANCELLED" || !got.LateCancellation ||
		got.CancellationReason == nil || *got.CancellationReason != "feeling unwell" {
		t.Errorf("unexpected response %+v", got)
	}
	if svc.gotCancel.id != appointmentID || svc.gotCancel.reason != "feeling unwell" {
		t.Errorf("service received %+v", svc.gotCancel)
	}
}

// check invalid req are rejected
func TestCancelAppointmentBadRequests(t *testing.T) {
	tests := []struct{ name, target, body string }{
		{"appointment id is not a UUID", "/appointments/abc/cancel", `{"reason":"x"}`},
		{"empty body", "/appointments/" + appointmentID + "/cancel", ``},
		{"not JSON", "/appointments/" + appointmentID + "/cancel", `reason: sick`},
		{"unknown field", "/appointments/" + appointmentID + "/cancel", `{"reason":"x","status":"CONFIRMED"}`},
		{"reason has the wrong type", "/appointments/" + appointmentID + "/cancel", `{"reason":123}`},
		{"trailing data", "/appointments/" + appointmentID + "/cancel", `{"reason":"x"}{}`},
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
			if svc.cancelCalled {
				t.Error("the service must not be called for an invalid request")
			}
		})
	}
}

// checks that service errors are mapped to the correct HTTP status codes
func TestCancelAppointmentMapsServiceErrors(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{domain.ErrReasonRequired, http.StatusBadRequest},
		{domain.ErrReasonTooLong, http.StatusBadRequest},
		{domain.ErrInvalidInput, http.StatusBadRequest},
		{domain.ErrNotFound, http.StatusNotFound},
		{domain.ErrAlreadyCancelled, http.StatusConflict},
		{domain.ErrAlreadyStarted, http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		t.Run(tc.err.Error(), func(t *testing.T) {
			rec := patch(t, &fakeService{cancelErr: tc.err}, "/appointments/"+appointmentID+"/cancel", `{"reason":"x"}`)
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

// checks missing reason is passed to the service, which will reject it with ErrReasonRequired
func TestCancelAppointmentMissingReasonReachesService(t *testing.T) {
	svc := &fakeService{cancelErr: domain.ErrReasonRequired}
	rec := patch(t, svc, "/appointments/"+appointmentID+"/cancel", `{}`)
	if rec.Code != http.StatusBadRequest || !svc.cancelCalled || svc.gotCancel.reason != "" {
		t.Errorf("status %d, called %v, reason %q; want 400 from the service's own reason check", rec.Code, svc.cancelCalled, svc.gotCancel.reason)
	}
}

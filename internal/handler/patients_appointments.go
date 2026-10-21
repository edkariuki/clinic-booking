package handler

import "net/http"

type patientAppointmentsResponse struct {
	PatientID    string                `json:"patient_id"`
	Appointments []appointmentResponse `json:"appointments"`
}

// patientAppointments handles GET /patients/{id}/appointments: the patient upcoming confirmed appointments, earliest first.
func (h *Handler) patientAppointments(w http.ResponseWriter, r *http.Request) {
	patientID := r.PathValue("id")

	list, err := h.svc.UpcomingForPatient(r.Context(), patientID)
	if err != nil {
		writeError(w, err)
		return
	}

	appointments := make([]appointmentResponse, 0, len(list))
	for _, a := range list {
		appointments = append(appointments, toAppointmentResponse(a))
	}
	writeJSON(w, http.StatusOK, patientAppointmentsResponse{
		PatientID:    patientID,
		Appointments: appointments,
	})
}

package domain

import "errors"

var (
	// 400
	ErrInvalidInput = errors.New("invalid input")

	// 404
	ErrNotFound = errors.New("not found")

	// 409
	ErrSlotTaken        = errors.New("this slot is already booked")
	ErrPatientBusy      = errors.New("the patient already has an appointment at that time")
	ErrAlreadyCancelled = errors.New("appointment is already cancelled")

	// 422
	ErrInPast              = errors.New("the requested time is in the past")
	ErrTooSoon             = errors.New("appointments must start at least 1 hour from now")
	ErrOutsideWorkingHours = errors.New("the requested time is outside the doctor's working hours")
	ErrMisaligned          = errors.New("start time must align to a 30-minute slot within the doctor's shift")
	ErrAlreadyStarted      = errors.New("the appointment has already started")
	ErrRescheduleTooLate   = errors.New("appointments starting within 1 hour cannot be rescheduled; cancel and book a new appointment instead")
	ErrReasonRequired      = errors.New("a cancellation reason is required")
	ErrReasonTooLong       = errors.New("cancellation reason must be at most 500 characters")
)

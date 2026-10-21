package domain

import "time"

const (
	SlotDuration    = 30 * time.Minute
	MinNotice       = time.Hour
	MaxReasonLength = 500
)

type Status string

const (
	StatusConfirmed Status = "CONFIRMED"
	StatusCancelled Status = "CANCELLED"
)

type Doctor struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type Shift struct {
	DoctorID  string
	DayOfWeek time.Weekday
	Start     time.Duration
	End       time.Duration
}

type Appointment struct {
	ID                 string
	DoctorID           string
	PatientID          string
	StartTime          time.Time
	EndTime            time.Time
	Status             Status
	CancellationReason *string
	LateCancellation   bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

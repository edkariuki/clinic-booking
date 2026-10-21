package service

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

var loc = domain.ClinicLocation

func at(day, hour, min int) time.Time {
	return time.Date(2026, time.October, day, hour, min, 0, 0, loc)
}

var (
	now = at(21, 9, 0)

	wed = []domain.Shift{
		{DayOfWeek: time.Wednesday, Start: 8 * time.Hour, End: 12 * time.Hour},
		{DayOfWeek: time.Wednesday, Start: 13 * time.Hour, End: 17 * time.Hour},
	}
)

func TestGenerateSlots(t *testing.T) {
	slots := GenerateSlots(wed, at(21, 0, 0))
	if len(slots) != 16 {
		t.Fatalf("want 16 slots (8 per shift), got %d", len(slots))
	}
	if !slots[0].Equal(at(21, 8, 0)) {
		t.Errorf("first slot = %v, want 08:00", slots[0])
	}
	if !slots[7].Equal(at(21, 11, 30)) || !slots[8].Equal(at(21, 13, 0)) {
		t.Errorf("lunch gap wrong: %v then %v", slots[7], slots[8])
	}
	if !slots[15].Equal(at(21, 16, 30)) {
		t.Errorf("last slot = %v, want 16:30", slots[15])
	}
}

func TestGenerateSlotsDayWithoutShifts(t *testing.T) {
	if got := GenerateSlots(wed, at(22, 12, 0)); len(got) != 0 {
		t.Errorf("want no slots on a day without shifts, got %d", len(got))
	}
}

func TestGenerateSlotsUsesClinicWeekday(t *testing.T) {
	tue2200UTC := time.Date(2026, time.October, 20, 22, 0, 0, 0, time.UTC)
	if got := GenerateSlots(wed, tue2200UTC); len(got) != 16 {
		t.Errorf("want Wednesday's 16 slots, got %d", len(got))
	}
}

func TestGenerateSlotsDropsPartialSlot(t *testing.T) {
	shifts := []domain.Shift{{DayOfWeek: time.Wednesday, Start: 8 * time.Hour, End: 8*time.Hour + 45*time.Minute}}
	if got := GenerateSlots(shifts, at(21, 0, 0)); len(got) != 1 {
		t.Errorf("a 45-minute shift holds one full slot, got %d", len(got))
	}
}

func TestAvailableSlots(t *testing.T) {
	taken := []time.Time{at(21, 10, 30), at(21, 13, 0)}
	got := AvailableSlots(wed, at(21, 0, 0), taken, now)

	// now is 09:00, earliest bookable slot is 10:00.
	if len(got) == 0 || !got[0].Equal(at(21, 10, 0)) {
		t.Fatalf("first available = %v, want 10:00", got)
	}
	for _, s := range got {
		if s.Equal(at(21, 10, 30)) || s.Equal(at(21, 13, 0)) {
			t.Errorf("taken slot %v must not be offered", s)
		}
		if s.Before(at(21, 10, 0)) {
			t.Errorf("slot %v is inside the 1-hour buffer", s)
		}
	}
	// 16 total - 4 before 10:00 (08:00,08:30,09:00,09:30) - 2 taken = 10
	if len(got) != 10 {
		t.Errorf("want 10 available slots, got %d", len(got))
	}
}

func TestAvailableSlotsEmptyIsNotNil(t *testing.T) {
	if got := AvailableSlots(wed, at(22, 0, 0), nil, now); got == nil {
		t.Error("want an empty, non-nil slice so JSON encodes as []")
	}
}

func TestAvailableSlotsFutureDayIgnoresBuffer(t *testing.T) {
	next := at(28, 0, 0)
	if got := AvailableSlots(wed, next, nil, now); len(got) != 16 {
		t.Errorf("want all 16 slots on a future day, got %d", len(got))
	}
}

func TestValidateNewSlot(t *testing.T) {
	short := []domain.Shift{{DayOfWeek: time.Wednesday, Start: 8 * time.Hour, End: 12*time.Hour + 15*time.Minute}}

	tests := []struct {
		name   string
		shifts []domain.Shift
		start  time.Time
		want   error
	}{
		{"valid, exactly 1 hour ahead", wed, at(21, 10, 0), nil},
		{"valid, later slot", wed, at(21, 16, 30), nil},
		{"valid, future week", wed, at(28, 8, 0), nil},
		{"valid when given as UTC", wed, time.Date(2026, 10, 21, 7, 0, 0, 0, time.UTC), nil},
		{"in the past", wed, at(21, 8, 0), domain.ErrInPast},
		{"yesterday", wed, at(20, 10, 0), domain.ErrInPast},
		{"inside the 1-hour buffer", wed, at(21, 9, 30), domain.ErrTooSoon},
		{"one minute short of the buffer", wed, at(21, 9, 59), domain.ErrTooSoon},
		{"during lunch break", wed, at(21, 12, 0), domain.ErrOutsideWorkingHours},
		{"after the last shift", wed, at(21, 17, 0), domain.ErrOutsideWorkingHours},
		{"before the first shift (next week)", wed, at(28, 7, 30), domain.ErrOutsideWorkingHours},
		{"day without shifts", wed, at(22, 10, 0), domain.ErrOutsideWorkingHours},
		{"not on the 30-minute grid", wed, at(21, 10, 15), domain.ErrMisaligned},
		{"grid is relative to the second shift", wed, at(21, 13, 45), domain.ErrMisaligned},
		{"slot would run past the shift end", short, at(21, 12, 0), domain.ErrOutsideWorkingHours},
		{"seconds make it misaligned", wed, at(21, 10, 0).Add(5 * time.Second), domain.ErrMisaligned},
		{"no shifts at all", nil, at(21, 10, 0), domain.ErrOutsideWorkingHours},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ValidateNewSlot(tc.shifts, tc.start, now)
			if !errors.Is(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCancellationOutcome(t *testing.T) {
	tests := []struct {
		name     string
		start    time.Time
		wantLate bool
		wantErr  error
	}{
		{"well ahead", at(21, 12, 0), false, nil},
		{"exactly 1 hour ahead is not late", at(21, 10, 0), false, nil},
		{"59 minutes ahead is late", at(21, 9, 59), true, nil},
		{"1 minute ahead is late", at(21, 9, 1), true, nil},
		{"starting right now", at(21, 9, 0), false, domain.ErrAlreadyStarted},
		{"already started", at(21, 8, 30), false, domain.ErrAlreadyStarted},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			late, err := CancellationOutcome(tc.start, now)
			if !errors.Is(err, tc.wantErr) || late != tc.wantLate {
				t.Errorf("got late=%v err=%v, want late=%v err=%v", late, err, tc.wantLate, tc.wantErr)
			}
		})
	}
}

func TestValidateReschedulable(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
		want  error
	}{
		{"well ahead", at(21, 12, 0), nil},
		{"exactly 1 hour ahead", at(21, 10, 0), nil},
		{"45 minutes ahead (09:45 at 09:00)", at(21, 9, 45), domain.ErrRescheduleTooLate},
		{"already started", at(21, 8, 30), domain.ErrAlreadyStarted},
		{"starting right now", at(21, 9, 0), domain.ErrAlreadyStarted},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ValidateReschedulable(tc.start, now); !errors.Is(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNormalizeReason(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   string
		wantEr error
	}{
		{"trims whitespace", "  feeling unwell \n", "feeling unwell", nil},
		{"empty", "", "", domain.ErrReasonRequired},
		{"only whitespace", "  \t ", "", domain.ErrReasonRequired},
		{"exactly 500 characters", strings.Repeat("a", 500), strings.Repeat("a", 500), nil},
		{"501 characters", strings.Repeat("a", 501), "", domain.ErrReasonTooLong},
		{"counts characters, not bytes", strings.Repeat("é", 500), strings.Repeat("é", 500), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeReason(tc.in)
			if !errors.Is(err, tc.wantEr) || got != tc.want {
				t.Errorf("got (%q, %v), want (%q, %v)", got, err, tc.want, tc.wantEr)
			}
		})
	}
}

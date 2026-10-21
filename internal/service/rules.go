package service

import (
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/edkariuki/clinic-booking/internal/domain"
)

func timeOfDay(t time.Time) time.Duration {
	l := t.In(domain.ClinicLocation)
	return time.Duration(l.Hour())*time.Hour +
		time.Duration(l.Minute())*time.Minute +
		time.Duration(l.Second())*time.Second +
		time.Duration(l.Nanosecond())
}

// GenerateSlots returns every 30min slot start for the day
func GenerateSlots(shifts []domain.Shift, day time.Time) []time.Time {
	local := day.In(domain.ClinicLocation)
	y, m, d := local.Date()

	var slots []time.Time
	for _, s := range shifts {
		if s.DayOfWeek != local.Weekday() {
			continue
		}
		for t := s.Start; t+domain.SlotDuration <= s.End; t += domain.SlotDuration {
			slots = append(slots, time.Date(y, m, d, 0, 0, int(t/time.Second), 0, domain.ClinicLocation))
		}
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].Before(slots[j]) })
	return slots
}

// AvailableSlots returns slots for the day that are not taken and that start at least MinNotice from now
func AvailableSlots(shifts []domain.Shift, day time.Time, taken []time.Time, now time.Time) []time.Time {
	takenSet := make(map[int64]struct{}, len(taken))
	for _, t := range taken {
		takenSet[t.Unix()] = struct{}{}
	}
	earliest := now.Add(domain.MinNotice)

	available := []time.Time{}
	for _, s := range GenerateSlots(shifts, day) {
		if s.Before(earliest) {
			continue
		}
		if _, isTaken := takenSet[s.Unix()]; isTaken {
			continue
		}
		available = append(available, s)
	}
	return available
}

// ValidateNewSlot implements the rules for booking a slot
func ValidateNewSlot(shifts []domain.Shift, start, now time.Time) error {
	if start.Before(now) {
		return domain.ErrInPast
	}
	if start.Sub(now) < domain.MinNotice {
		return domain.ErrTooSoon
	}

	local := start.In(domain.ClinicLocation)
	tod := timeOfDay(start)
	for _, s := range shifts {
		if s.DayOfWeek != local.Weekday() || tod < s.Start || tod >= s.End {
			continue
		}
		if (tod-s.Start)%domain.SlotDuration != 0 {
			return domain.ErrMisaligned
		}
		if tod+domain.SlotDuration > s.End {
			return domain.ErrOutsideWorkingHours
		}
		return nil
	}
	return domain.ErrOutsideWorkingHours
}

// CancellationOutcome decides whether an appointment can be cancelled at now.
func CancellationOutcome(start, now time.Time) (late bool, err error) {
	if !now.Before(start) {
		return false, domain.ErrAlreadyStarted
	}
	return start.Sub(now) < domain.MinNotice, nil
}

// ValidateReschedulable checks the existing appointment before it is rescheduled
func ValidateReschedulable(currentStart, now time.Time) error {
	if !now.Before(currentStart) {
		return domain.ErrAlreadyStarted
	}
	if currentStart.Sub(now) < domain.MinNotice {
		return domain.ErrRescheduleTooLate
	}
	return nil
}

// NormalizeReason checks cancallation reason chracters removing whitespace
func NormalizeReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "", domain.ErrReasonRequired
	}
	if utf8.RuneCountInString(reason) > domain.MaxReasonLength {
		return "", domain.ErrReasonTooLong
	}
	return reason, nil
}

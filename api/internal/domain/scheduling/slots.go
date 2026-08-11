package scheduling

import "time"

// SlotRequest carries everything the slot engine needs, as plain domain
// values. Busy holds existing bookings as intervals so the engine stays
// independent of the booking entity arriving in BE-05.
type SlotRequest struct {
	Rules           []WeeklyRule
	TimeOff         []Interval
	Busy            []Interval
	DurationMinutes int
	// From and To are inclusive calendar dates interpreted in Loc.
	From time.Time
	To   time.Time
	// Loc is the timezone the schedule is evaluated in. Results are
	// always emitted in UTC.
	Loc *time.Location
	// Now excludes starts in the past.
	Now time.Time
}

// GenerateSlots computes the bookable start times for a service over a
// date range. It evaluates windows in the local wall clock of Loc (DST-safe)
// and returns half-open UTC intervals [start, start+duration).
//
// Candidate starts begin at each window start and step by
// DurationMinutes + the weekday's BufferMinutes. A candidate is bookable
// when it fits inside its window, starts at or after Now, overlaps no
// time-off, and overlaps no busy interval expanded by BufferMinutes on
// both sides.
//
// Wall-clock note: windows are anchored with time.Date in Loc, so a
// 09:00 window opens at 09:00 local on both sides of a DST transition.
// A window edge inside the spring-forward gap is interpreted in the
// pre-transition offset per the Go stdlib, effectively shifting it past
// the gap — practitioners should not schedule 2am windows.
func GenerateSlots(req SlotRequest) ([]Interval, error) {
	if req.DurationMinutes < 1 {
		return nil, ErrInvalidDuration
	}
	if req.Loc == nil {
		return nil, ErrInvalidTimezone
	}
	if err := ValidateRules(req.Rules); err != nil {
		return nil, err
	}

	startDay := midnight(req.From.In(req.Loc))
	endDay := midnight(req.To.In(req.Loc))
	if endDay.Before(startDay) {
		return nil, ErrInvalidRange
	}
	// Calendar arithmetic, not elapsed hours: DST days are 23/25 hours.
	if endDay.After(startDay.AddDate(0, 0, MaxSlotRangeDays)) {
		return nil, ErrInvalidRange
	}

	rulesByWeekday := make(map[time.Weekday]WeeklyRule, len(req.Rules))
	for _, r := range req.Rules {
		rulesByWeekday[r.Weekday] = r
	}

	duration := time.Duration(req.DurationMinutes) * time.Minute
	now := req.Now.UTC()
	var slots []Interval

	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		rule, open := rulesByWeekday[day.Weekday()]
		if !open {
			continue
		}
		buffer := time.Duration(rule.BufferMinutes) * time.Minute
		step := duration + buffer

		for _, w := range rule.Windows {
			winStart := wallClock(day, w.StartMin, req.Loc)
			winEnd := wallClock(day, w.EndMin, req.Loc)
			for start := winStart; ; start = start.Add(step) {
				slot := Interval{Start: start, End: start.Add(duration)}
				if slot.End.After(winEnd) {
					break
				}
				if slot.Start.Before(now) {
					continue
				}
				if blocked(slot, req.TimeOff, req.Busy, buffer) {
					continue
				}
				slots = append(slots, Interval{Start: slot.Start.UTC(), End: slot.End.UTC()})
			}
		}
	}
	return slots, nil
}

// blocked reports whether a candidate slot collides with time-off or with
// any busy interval expanded by the day's buffer on both sides.
func blocked(slot Interval, timeOff, busy []Interval, buffer time.Duration) bool {
	for _, off := range timeOff {
		if slot.Overlaps(off) {
			return true
		}
	}
	for _, b := range busy {
		expanded := Interval{Start: b.Start.Add(-buffer), End: b.End.Add(buffer)}
		if slot.Overlaps(expanded) {
			return true
		}
	}
	return false
}

// midnight truncates t (already in the target location) to local midnight.
func midnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// wallClock anchors minutes-since-midnight to the local wall clock of loc
// on the given day, so windows track civil time across DST transitions.
func wallClock(day time.Time, min int, loc *time.Location) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), min/60, min%60, 0, 0, loc)
}

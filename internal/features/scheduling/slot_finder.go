package scheduling

import (
	"appointments/internal/features/services"
	"context"
	"time"

	"github.com/google/uuid"
)

type SlotFinder struct {
	availRepo AvailabilityRepository
}

func NewSlotFinder(repo AvailabilityRepository) *SlotFinder {
	return &SlotFinder{availRepo: repo}
}

func (sf *SlotFinder) GetAvailableSlots(ctx context.Context, resourceID uuid.UUID, date time.Time, service *services.Service) ([]TimeSlot, error) {
	override, err := sf.availRepo.GetOverrides(ctx, resourceID, date)
	if err != nil {
		return nil, err
	}
	if override != nil && override.IsDayOff {
		return nil, nil
	}

	workStart, workEnd, err := sf.resolveWorkingHours(ctx, resourceID, date, override)
	if err != nil {
		return nil, err
	}
	if workStart == nil {
		return nil, nil // day off
	}

	occupied, err := sf.availRepo.GetOccupiedSlots(ctx, resourceID, date)
	if err != nil {
		return nil, err
	}

	slotDuration := time.Duration(service.DurationMinutes+service.BufferMinutes) * time.Minute
	var available []TimeSlot

	current := *workStart
	for current.Add(time.Duration(service.DurationMinutes)*time.Minute).Before(*workEnd) ||
		current.Add(time.Duration(service.DurationMinutes)*time.Minute).Equal(*workEnd) {

		slotEnd := current.Add(slotDuration)
		if !sf.overlapsAny(current, slotEnd, occupied) {
			available = append(available, TimeSlot{
				StartsAt:   current,
				EndsAt:     current.Add(time.Duration(service.DurationMinutes) * time.Minute),
				ResourceID: resourceID,
			})
		}
		current = current.Add(slotDuration)
	}

	return available, nil
}

func (sf *SlotFinder) GetSuggestedSlots(ctx context.Context, resourceID uuid.UUID, from time.Time, service *services.Service) ([]TimeSlot, error) {
	var suggestions []TimeSlot
	current := from
	maxDays := 7

	for len(suggestions) < 3 && current.Before(from.AddDate(0, 0, maxDays)) {
		slots, err := sf.GetAvailableSlots(ctx, resourceID, current, service)
		if err != nil {
			return nil, err
		}

		for _, slot := range slots {
			if slot.StartsAt.After(from) {
				suggestions = append(suggestions, slot)
				if len(suggestions) == 3 {
					break
				}
			}
		}

		next := current.AddDate(0, 0, 1)
		current = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, current.Location())
	}

	return suggestions, nil
}

func (sf *SlotFinder) resolveWorkingHours(ctx context.Context, resourceID uuid.UUID, date time.Time, override *ScheduleOverride) (*time.Time, *time.Time, error) {
	loc := date.Location()

	if override != nil && !override.IsDayOff && override.StartTime != nil {
		start := parseTimeOnDate(date, *override.StartTime, loc)
		end := parseTimeOnDate(date, *override.EndTime, loc)
		return &start, &end, nil
	}

	weeklyHours, err := sf.availRepo.GetWorkingHours(ctx, resourceID)
	if err != nil {
		return nil, nil, err
	}

	dow := int(date.Weekday()) // 0=Sunday
	for _, wh := range weeklyHours {
		if wh.DayOfWeek == dow && wh.StartTime != "" {
			start := parseTimeOnDate(date, wh.StartTime, loc)
			end := parseTimeOnDate(date, wh.EndTime, loc)
			return &start, &end, nil
		}
	}

	return nil, nil, nil
}

func (sf *SlotFinder) overlapsAny(start, end time.Time, occupied []TimeSlot) bool {
	for _, o := range occupied {
		if start.Before(o.EndsAt) && end.After(o.StartsAt) {
			return true
		}
	}
	return false
}

func parseTimeOnDate(date time.Time, t string, loc *time.Location) time.Time {
	parsed, _ := time.Parse("15:04:05", t)
	return time.Date(date.Year(), date.Month(), date.Day(),
		parsed.Hour(), parsed.Minute(), 0, 0, loc)
}

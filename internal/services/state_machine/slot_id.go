package state_machine

import (
	"fmt"
	"strings"
	"time"
	"wappiz/internal/services/slot_finder"

	"github.com/google/uuid"
)

func parseSlotID(id string) (time.Time, uuid.UUID, error) {
	parts := strings.SplitN(id, "_", 3)
	if len(parts) != 3 {
		return time.Time{}, uuid.UUID{}, fmt.Errorf("invalid slot id")
	}
	t, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return time.Time{}, uuid.UUID{}, err
	}
	rid, err := uuid.Parse(parts[2])
	if err != nil {
		return time.Time{}, uuid.UUID{}, err
	}
	return t, rid, nil
}

func buildSlotID(slot slot_finder.TimeSlot) string {
	return fmt.Sprintf("slot_%s_%s", slot.StartsAt.UTC().Format(time.RFC3339), slot.ResourceID)
}

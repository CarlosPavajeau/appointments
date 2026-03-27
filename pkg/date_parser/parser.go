package date_parser

import (
	"fmt"
	"strings"
	"time"
	apperrors "wappiz/pkg/errors"
)

var formats = []string{
	"02/01 03:04 PM",
	"02/01 3:04 PM",
	"02/01 03:04PM",
	"02/01 3:04PM",
	"02/01 03:04 AM",
	"02/01 3:04 AM",
}

func ParseDateTime(input string, loc *time.Location) (time.Time, error) {
	input = strings.TrimSpace(input)
	input = strings.Join(strings.Fields(input), " ")
	input = strings.ToUpper(input)

	now := time.Now().In(loc)
	year := now.Year()

	for _, format := range formats {
		fullFormat := fmt.Sprintf("2006/%s", format)
		fullInput := fmt.Sprintf("%d/%s", year, input)

		t, err := time.ParseInLocation(fullFormat, fullInput, loc)
		if err != nil {
			continue
		}

		if t.Before(now.Truncate(24 * time.Hour)) {
			t = t.AddDate(1, 0, 0)
		}

		return t, nil
	}

	return time.Time{}, apperrors.ErrInvalidFormat
}

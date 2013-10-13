package store

import (
	"strconv"
	"time"
)

// Convert string duration to time format
func TTL(duration string) (time.Time, error) {
	if duration != "" {
		duration, err := strconv.Atoi(duration)
		if err != nil {
			return Permanent, err
		}
		return time.Now().Add(time.Second * (time.Duration)(duration)), nil

	} else {
		return Permanent, nil
	}
}

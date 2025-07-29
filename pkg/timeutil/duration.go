package timeutil

import (
	"encoding/json"
	"time"
)

type Duration struct {
	time.Duration
}

func FromTimeDuration(duration time.Duration) Duration {
	return Duration{duration}
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	var err error
	if err = json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		d.Duration = time.Duration(value)
		return nil
	case string:
		d.Duration, err = time.ParseDuration(value)

		return err
	}
	return err
}

func (d *Duration) Set(s string) error {
	td, err := time.ParseDuration(s)
	if err == nil {
		d.Duration = td
	}
	return err
}

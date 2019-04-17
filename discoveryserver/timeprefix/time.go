package timeprefix

import "time"

func Prefixes(before time.Duration, durationrange time.Duration) []string {
	ft := time.Now().UTC().Add(-1 * (before + durationrange))
	lt := time.Now().UTC().Add(-1 * before)

	return []string{ft.Format(time.RFC3339), lt.Format(time.RFC3339)}
}

func Now() string {
	return Past(0)
}

func Past(d time.Duration) string {
	return time.Now().Add(-1 * d).UTC().Format(time.RFC3339)
}

package server

import (
	"time"
)

// packageStats represent the stats we need for a package.
// It has sending time and the size of the package.
type packageStats struct {
	sendingTime time.Time
	size        int
}

// NewPackageStats creates a pacakgeStats and return the pointer to it.
func NewPackageStats(now time.Time, size int) *packageStats {
	return &packageStats{
		sendingTime: now,
		size:        size,
	}
}

// Time return the sending time of the package.
func (ps *packageStats) Time() time.Time {
	return ps.sendingTime
}

package utils

import (
	"syscall"
	"time"
)

var (
	lastInspectUnixNano int64
	lastCPUUsageTime    int64
)

// GetCPUPercentage calculates CPU usage and returns percentage in float64(e.g. 2.5 means 2.5%).
// http://man7.org/linux/man-pages/man2/getrusage.2.html
func GetCPUPercentage() float64 {
	var ru syscall.Rusage
	syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
	usageTime := ru.Utime.Nano() + ru.Stime.Nano()
	nowTime := time.Now().UnixNano()
	perc := float64(usageTime-lastCPUUsageTime) / float64(nowTime-lastInspectUnixNano) * 100.0
	lastInspectUnixNano = nowTime
	lastCPUUsageTime = usageTime
	return perc
}

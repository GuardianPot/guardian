//go:build linux

package health

import (
	"time"

	"golang.org/x/sys/unix"
)

func clockMeasurement() (available, synchronized bool, offset time.Duration) {
	var measurement unix.Timex
	state, err := unix.Adjtimex(&measurement)
	if err != nil {
		return false, false, 0
	}
	offset = time.Duration(measurement.Offset) * time.Microsecond
	if measurement.Status&unix.STA_NANO != 0 {
		offset = time.Duration(measurement.Offset)
	}
	return true, state != unix.TIME_ERROR && measurement.Status&unix.STA_UNSYNC == 0, offset
}

func filesystemFreePercent(path string) (float64, bool) {
	var stats unix.Statfs_t
	if err := unix.Statfs(path, &stats); err != nil || stats.Blocks == 0 {
		return 0, false
	}
	return float64(stats.Bavail) * 100 / float64(stats.Blocks), true
}

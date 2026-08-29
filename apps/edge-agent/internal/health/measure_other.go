//go:build !linux

package health

import "time"

func clockMeasurement() (bool, bool, time.Duration) { return false, false, 0 }

func filesystemFreePercent(string) (float64, bool) { return 0, false }

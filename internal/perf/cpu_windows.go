//go:build windows

package perf

import (
	"time"

	"golang.org/x/sys/windows"
)

func processCPUTime() (time.Duration, error) {
	var creationTime windows.Filetime
	var exitTime windows.Filetime
	var kernelTime windows.Filetime
	var userTime windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &creationTime, &exitTime, &kernelTime, &userTime); err != nil {
		return 0, err
	}
	return time.Duration(kernelTime.Nanoseconds() + userTime.Nanoseconds()), nil
}

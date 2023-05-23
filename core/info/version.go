package info

import (
	"runtime"
)

func OS() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	default:
		return runtime.GOOS
	}
}

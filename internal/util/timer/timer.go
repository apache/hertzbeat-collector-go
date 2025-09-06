package timer

import "time"

// GetCurrentTimeMillis returns current time in milliseconds
func GetCurrentTimeMillis() int64 {
	return time.Now().UnixMilli()
}

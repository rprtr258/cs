package internal

import "time"

// Returns the current time as a millisecond timestamp
func nowMillis() int64 {
	return time.Now().UnixMilli()
}

// Returns the current time as a nanosecond timestamp as some things
// are far too fast to measure using nanoseconds
func nowNanos() int64 {
	return time.Now().UnixNano()
}

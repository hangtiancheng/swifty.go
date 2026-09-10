package redis

import "time"

// secondsToDuration converts a duration in seconds to time.Duration.
func secondsToDuration(seconds int64) time.Duration {
	return time.Duration(seconds) * time.Second
}

// millisToDuration converts a duration in milliseconds to time.Duration.
func millisToDuration(millis int64) time.Duration {
	return time.Duration(millis) * time.Millisecond
}

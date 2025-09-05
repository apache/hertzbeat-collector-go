package err

import "errors"

// Collector Server Error Types
var (
	CollectorIPIsNull   = errors.New("collector ip is empty")
	CollectorPortIsNull = errors.New("collector port is empty")
)

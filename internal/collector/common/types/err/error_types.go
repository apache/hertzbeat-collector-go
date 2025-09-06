package err

import "errors"

// Collector Server Error Types
var (
	CollectorConfigIsNil = errors.New("collector config is nil")
	CollectorIPIsNil     = errors.New("collector ip is empty")
	CollectorPortIsNil   = errors.New("collector port is empty")
	CollectorServerStop  = errors.New("collector server stop")
)

// Collector Banner Error Types
var (
	BannerPrintReaderError  = errors.New("print banner error")
	BannerPrintExecuteError = errors.New("print banner execute error")
)

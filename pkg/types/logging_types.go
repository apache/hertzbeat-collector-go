package types

// hertzbeat logging related types

type LogLevel string

const (
	// LogLevelTrace defines the "Trace" logging level.
	LogLevelTrace LogLevel = "trace"

	// LogLevelDebug defines the "debug" logging level.
	LogLevelDebug LogLevel = "debug"

	// LogLevelInfo defines the "Info" logging level.
	LogLevelInfo LogLevel = "info"

	// LogLevelWarn defines the "Warn" logging level.
	LogLevelWarn LogLevel = "warn"

	// LogLevelError defines the "Error" logging level.
	LogLevelError LogLevel = "error"
)

type HertzBeatLogging struct {
	Level map[HertzbeatLogComponent]LogLevel `json:"level,omitempty"`
}

type HertzbeatLogComponent string

const (
	LogComponentHertzbeatDefault HertzbeatLogComponent = "default"

	LogComponentHertzbeatCollector HertzbeatLogComponent = "collector"
)

func DefaultHertzbeatLogging() *HertzBeatLogging {

	return &HertzBeatLogging{
		Level: map[HertzbeatLogComponent]LogLevel{
			LogComponentHertzbeatDefault: LogLevelInfo,
		},
	}
}

func (logging *HertzBeatLogging) DefaultHertzBeatLoggingLevel(level LogLevel) LogLevel {

	if level != "" {
		return level
	}

	if logging.Level[LogComponentHertzbeatDefault] != "" {

		return logging.Level[LogComponentHertzbeatDefault]
	}

	return LogLevelInfo
}

func (logging *HertzBeatLogging) SetHertzBeatLoggingDefaults() {

	if logging != nil && logging.Level != nil && logging.Level[LogComponentHertzbeatDefault] == "" {

		logging.Level[LogComponentHertzbeatDefault] = LogLevelInfo
	}
}

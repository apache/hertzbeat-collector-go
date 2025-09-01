package types

type CollectorConfig struct {
	Collector CollectorSection `yaml:"collector"`
}

type CollectorSection struct {
	Info CollectorInfo      `yaml:"info"`
	Log  CollectorLogConfig `yaml:"log"`
	// Add Dispatcher if needed
}

type CollectorInfo struct {
	Name string `yaml:"name"`
	IP   string `yaml:"ip"`
	Port string `yaml:"port"`
}

type CollectorLogConfig struct {
	Level string `yaml:"level"`
}

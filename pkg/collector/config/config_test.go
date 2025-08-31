package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {

	content := `{
        "collector": {
            "version": "1.0.0",
            "ip": "127.0.0.1"
        },
        "dispatcher": {}
    }`
	dumpfile, err := os.CreateTemp("", "collector_config_*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			t.Logf("failed to remove temp file: %v", err)
		}
	}(dumpfile.Name())
	if _, err := dumpfile.Write([]byte(content)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	err = dumpfile.Close()
	if err != nil {
		return
	}

	cfg, err := LoadConfig(dumpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Collector.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", cfg.Collector.Version)
	}
	if cfg.Collector.IP != "127.0.0.1" {
		t.Errorf("expected ip '127.0.0.1', got '%s'", cfg.Collector.IP)
	}
}

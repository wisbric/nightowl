package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	tests := []struct {
		name   string
		check  func(*Config) bool
		expect string
	}{
		{
			name:   "default mode is api",
			check:  func(c *Config) bool { return c.Mode == "api" },
			expect: "api",
		},
		{
			name:   "default host is 0.0.0.0",
			check:  func(c *Config) bool { return c.Host == "0.0.0.0" },
			expect: "0.0.0.0",
		},
		{
			name:   "default port is 8080",
			check:  func(c *Config) bool { return c.Port == 8080 },
			expect: "8080",
		},
		{
			name:   "default log level is info",
			check:  func(c *Config) bool { return c.LogLevel == "info" },
			expect: "info",
		},
		{
			name:   "default log format is json",
			check:  func(c *Config) bool { return c.LogFormat == "json" },
			expect: "json",
		},
		{
			name:   "default metrics path",
			check:  func(c *Config) bool { return c.MetricsPath == "/metrics" },
			expect: "/metrics",
		},
		{
			name:   "listen addr format",
			check:  func(c *Config) bool { return c.ListenAddr() == "0.0.0.0:8080" },
			expect: "0.0.0.0:8080",
		},
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(cfg) {
				t.Errorf("expected %s", tt.expect)
			}
		})
	}
}

package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Target   string          `yaml:"target"`
	Strategy string          `yaml:"strategy"`
	Rounds   int             `yaml:"rounds"`
	Delay    time.Duration   `yaml:"delay"`
	HTTP2    bool            `yaml:"http2"`
	Timeout  time.Duration   `yaml:"timeout"`
	Requests []RequestConfig `yaml:"requests"`
}

type RequestConfig struct {
	Name    string            `yaml:"name"`
	Method  string            `yaml:"method"`
	URL     string            `yaml:"url"`
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers"`
	Body    string            `yaml:"body"`
	Count   int               `yaml:"count"`
}

func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Strategy: "barrier",
		Rounds:   1,
		Delay:    time.Second,
		Timeout:  10 * time.Second,
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	for i := range cfg.Requests {
		if cfg.Requests[i].Method == "" {
			cfg.Requests[i].Method = "GET"
		}
		if cfg.Requests[i].Count == 0 {
			cfg.Requests[i].Count = 20
		}
		if cfg.Requests[i].URL == "" && cfg.Requests[i].Path != "" {
			cfg.Requests[i].URL = cfg.Target + cfg.Requests[i].Path
		}
	}

	return cfg, nil
}

package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Endpoints []Endpoint `yaml:"endpoints"`
}

type Endpoint struct {
	Name       string            `yaml:"name"`
	URL        string            `yaml:"url"`
	Method     string            `yaml:"method"`
	Headers    map[string]string `yaml:"headers"`
	Body       string            `yaml:"body"`
	Expect     Expectation       `yaml:"expect"`
	Retry      RetryConfig       `yaml:"retry"`
	Concurrent ConcurrentConfig  `yaml:"concurrent"`
}

type Expectation struct {
	Status  int           `yaml:"status"`
	MaxTime time.Duration `yaml:"maxTime"`
	Values  []ValueCheck  `yaml:"values"`
}

type ValueCheck struct {
	Path  string      `yaml:"path"`
	Value interface{} `yaml:"value"`
}

type RetryConfig struct {
	Count int           `yaml:"count"`
	Delay time.Duration `yaml:"delay"`
}

type ConcurrentConfig struct {
	Users int           `yaml:"users"`
	Delay time.Duration `yaml:"delay"`
	Total int           `yaml:"total"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func (c *Config) Validate() error {
	if len(c.Endpoints) == 0 {
		return fmt.Errorf("no endpoints defined")
	}

	for _, e := range c.Endpoints {
		if e.URL == "" {
			return fmt.Errorf("endpoint %s: missing URL", e.Name)
		}
		if e.Method == "" {
			return fmt.Errorf("endpoint %s: missing method", e.Name)
		}
		if e.Concurrent.Users > 0 && e.Concurrent.Total == 0 {
			return fmt.Errorf("endpoint %s: concurrent users set but total requests not specified", e.Name)
		}
	}
	return nil
}

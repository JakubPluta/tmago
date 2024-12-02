package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Representation of the config file
type Config struct {
	Endpoints []Endpoint `yaml:"endpoints"`
}

// Representation of an endpoint in the config
// It's main object that is used to run the tests
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

// Representation of the expected response
type Expectation struct {
	Status  int           `yaml:"status"`
	MaxTime time.Duration `yaml:"maxTime"`
	Values  []ValueCheck  `yaml:"values"`
}

// Check if the response matches the expected values
type ValueCheck struct {
	Path  string      `yaml:"path"`
	Value interface{} `yaml:"value"`
}

// Representation of the retry configuration
type RetryConfig struct {
	Count int           `yaml:"count"`
	Delay time.Duration `yaml:"delay"`
}

// Representation of the concurrent configuration
type ConcurrentConfig struct {
	Users int           `yaml:"users"`
	Delay time.Duration `yaml:"delay"`
	Total int           `yaml:"total"`
}

// LoadConfig loads a configuration from a YAML file at the given path.
// It returns an error if the file cannot be read or if the YAML is invalid.
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
		log.Println("no endpoints defined")
		return fmt.Errorf("no endpoints defined")
	}

	for _, e := range c.Endpoints {
		if e.URL == "" {
			log.Println("endpoint", e.Name, "missing URL")
			return fmt.Errorf("endpoint %s: missing URL", e.Name)
		}
		if e.Method == "" {
			log.Println("endpoint", e.Name, "missing method")
			return fmt.Errorf("endpoint %s: missing method", e.Name)
		}
		if e.Concurrent.Users > 0 && e.Concurrent.Total == 0 {
			log.Println("endpoint", e.Name, "concurrent users set but total requests not specified")
			return fmt.Errorf("endpoint %s: concurrent users set but total requests not specified", e.Name)
		}
	}
	return nil
}

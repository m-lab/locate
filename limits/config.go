package limits

import (
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// AgentConfig holds the limit configuration for a user agent.
type AgentConfig struct {
	Agent    string        `yaml:"agent"`
	Schedule string        `yaml:"schedule"`
	Duration time.Duration `yaml:"duration"`
}

// Config holds the limit configuration for all user agents.
type Config []AgentConfig

// ParseConfig interprets the configuration file and returns the set
// of agent limits.
func ParseConfig(path string) (Agents, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	config := &Config{}
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(config)

	lmts := make(Agents)
	for _, l := range *config {
		lmts[l.Agent] = NewCron(l.Schedule, l.Duration)
	}
	return lmts, err
}

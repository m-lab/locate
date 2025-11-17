package limits

import (
	"fmt"
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

// TierLimitConfig holds the rate limit configuration for a single tier.
type TierLimitConfig struct {
	Interval  string `yaml:"interval"`
	MaxEvents int    `yaml:"max_events"`
}

// FullConfig holds the complete limit configuration including both
// agent-based and tier-based limits.
type FullConfig struct {
	Agents []AgentConfig              `yaml:"agents"`
	Tiers  map[int]TierLimitConfig    `yaml:"tiers"`
}

// Config holds the limit configuration for all user agents.
type Config []AgentConfig

// TierLimits maps tier numbers to their LimitConfig.
type TierLimits map[int]LimitConfig

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

// ParseFullConfig interprets the configuration file and returns both
// agent limits and tier limits.
func ParseFullConfig(path string) (Agents, TierLimits, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	config := &FullConfig{}
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(config); err != nil {
		return nil, nil, err
	}

	// Parse agent limits
	agentLimits := make(Agents)
	for _, l := range config.Agents {
		agentLimits[l.Agent] = NewCron(l.Schedule, l.Duration)
	}

	// Parse tier limits
	tierLimits := make(TierLimits)
	for tier, cfg := range config.Tiers {
		interval, err := time.ParseDuration(cfg.Interval)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid interval for tier %d: %w", tier, err)
		}
		tierLimits[tier] = LimitConfig{
			Interval:  interval,
			MaxEvents: cfg.MaxEvents,
		}
	}

	return agentLimits, tierLimits, nil
}

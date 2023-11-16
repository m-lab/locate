package limits

import (
	"time"

	"github.com/aptible/supercronic/cronexpr"
)

const (
	duration = 10 * time.Minute // Time limits last 10 minutes.
)

// NewCron returns a new instance of Cron.
func NewCron(schedule string) *Cron {
	return &Cron{
		Expression: cronexpr.MustParse(schedule),
	}
}

// Cron infers time limits based on a cron schedule.
type Cron struct {
	*cronexpr.Expression
}

// IsLimited returns whether the input time is within a time-limited
// window [start, end).
func (c *Cron) IsLimited(t time.Time) bool {
	start := c.Next(t.Add(-duration))
	end := start.Add(duration)
	return (t.Equal(start) || t.After(start)) && t.Before(end)
}

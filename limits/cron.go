package limits

import (
	"time"

	"github.com/aptible/supercronic/cronexpr"
)

// NewCron returns a new instance of Cron.
func NewCron(schedule string, duration time.Duration) *Cron {
	return &Cron{
		Expression: cronexpr.MustParse(schedule),
		duration:   duration,
	}
}

// Cron infers time limits based on a cron schedule.
type Cron struct {
	*cronexpr.Expression
	duration time.Duration
}

// IsLimited returns whether the input time is within a time-limited
// window [start, end).
func (c *Cron) IsLimited(t time.Time) bool {
	start := c.Next(t.Add(-c.duration))
	end := start.Add(c.duration)
	return (t.Equal(start) || t.After(start)) && t.Before(end)
}

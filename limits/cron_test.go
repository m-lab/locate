package limits

import (
	"testing"
	"time"
)

func TestSchedule_IsLimited(t *testing.T) {
	tests := []struct {
		name     string
		schedule string
		t        time.Time
		want     bool
	}{
		{
			name:     "within-limit",
			schedule: "15,45 5-11 * * *",                                         // At minute 15 and 45, every hour from 5 to 11.
			t:        time.Date(2023, time.November, 16, 10, 15, 0, 0, time.UTC), // At minute 15.
			want:     true,
		},
		{
			name:     "always",
			schedule: "* * * * *", // Every minute.
			t:        time.Now().UTC(),
			want:     true,
		},
		{
			name:     "outside-limit",
			schedule: "15,45 5-11 * * *",                                         // At minute 15 and 45, every hour from 5 to 11.
			t:        time.Date(2023, time.November, 16, 10, 25, 0, 0, time.UTC), // At minute 25.
			want:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCron(tt.schedule)
			if got := c.IsLimited(tt.t); got != tt.want {
				t.Errorf("Cron.IsLimited() = %v, want %v", got, tt.want)
			}
		})
	}
}

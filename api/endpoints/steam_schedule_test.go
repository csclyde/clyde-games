package endpoints

import (
	"testing"
	"time"
)

func TestNextSteamImportTime(t *testing.T) {
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		now  time.Time
		want time.Time
	}{
		{
			name: "before six schedules today",
			now:  time.Date(2026, time.January, 10, 5, 30, 0, 0, location),
			want: time.Date(2026, time.January, 10, 6, 0, 0, 0, location),
		},
		{
			name: "at six schedules tomorrow",
			now:  time.Date(2026, time.January, 10, 6, 0, 0, 0, location),
			want: time.Date(2026, time.January, 11, 6, 0, 0, 0, location),
		},
		{
			name: "after six schedules tomorrow across daylight saving",
			now:  time.Date(2026, time.March, 7, 7, 0, 0, 0, location),
			want: time.Date(2026, time.March, 8, 6, 0, 0, 0, location),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := nextSteamImportTime(test.now, location); !got.Equal(test.want) {
				t.Fatalf("got %s, want %s", got, test.want)
			}
		})
	}
}

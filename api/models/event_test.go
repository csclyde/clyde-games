package models

import "testing"

func TestBuildIsBefore(t *testing.T) {
	tests := []struct {
		name   string
		build  string
		oldest string
		want   bool
	}{
		{"older", "2025-01-01T00:00:00Z", "2025-02-01T00:00:00Z", true},
		{"same", "2025-02-01T00:00:00Z", "2025-02-01T00:00:00Z", false},
		{"newer", "2025-03-01T00:00:00Z", "2025-02-01T00:00:00Z", false},
		{"offsets", "2025-02-01T00:00:00-05:00", "2025-02-01T04:00:00Z", false},
		{"iso without timezone", "2025-01-01T12:30:00", "2025-01-02T12:30:00", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := buildIsBefore(test.build, test.oldest)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("buildIsBefore() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestBuildIsBeforeRejectsInvalidDate(t *testing.T) {
	if _, err := buildIsBefore("not-a-date", "2025-02-01T00:00:00Z"); err == nil {
		t.Fatal("expected an invalid event build to return an error")
	}
}

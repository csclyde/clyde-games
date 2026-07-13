package models

import (
	"encoding/json"
	"testing"
)

func TestEventMetricsPayload(t *testing.T) {
	payload := []byte(`{
		"Type": "clear_level",
		"Resources": {"Essence": 21, "Tears": 8},
		"Actions": {"Attack": 13, "Spell": 5, "Ward": 3, "Ult": 2, "Botyl": 1}
	}`)

	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}

	if event.Resources.Essence != 21 || event.Resources.Tears != 8 {
		t.Fatalf("unexpected resources: %+v", event.Resources)
	}
	if event.Actions.Attack != 13 || event.Actions.Spell != 5 || event.Actions.Ward != 3 || event.Actions.Ult != 2 || event.Actions.Botyl != 1 {
		t.Fatalf("unexpected actions: %+v", event.Actions)
	}
}

func TestLegacyEventMetricsPayload(t *testing.T) {
	payload := []byte(`{"Type": "load_level", "Level": "test_level"}`)

	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}

	if event.Resources != (EventResources{}) {
		t.Fatalf("expected omitted resources to default to zero values, got %+v", event.Resources)
	}
	if event.Actions != (EventActions{}) {
		t.Fatalf("expected omitted actions to default to zero values, got %+v", event.Actions)
	}
}

func TestEventMetricsJSONShape(t *testing.T) {
	event := Event{
		Resources: EventResources{Essence: 4, Tears: 2},
		Actions:   EventActions{Attack: 1},
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if string(decoded["Resources"]) != `{"Essence":4,"Tears":2}` {
		t.Fatalf("unexpected Resources JSON: %s", decoded["Resources"])
	}
	if string(decoded["Actions"]) != `{"Attack":1,"Spell":0,"Ward":0,"Ult":0,"Botyl":0}` {
		t.Fatalf("unexpected Actions JSON: %s", decoded["Actions"])
	}
}

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

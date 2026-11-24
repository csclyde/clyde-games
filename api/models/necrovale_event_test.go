package models

import (
	"encoding/json"
	"testing"
)

func TestNecrovaleRunPayload(t *testing.T) {
	raw := `{"PID":"install","SID":"session","Env":"prod","Project":"necrovale","Platform":"windows","Build":"2026-11-24T17:48:00","Commit":"e595535","Type":"player_died","Level":"SomeRoom","Area":"Forest","Mode":"roguelike","Difficulty":2.5,"RunStep":0,"Items":[{"Item":"TheLongestMeldedItemInThisFixture","Slot":"Weapon","Level":300,"Parent":"ExistingLongParentItemName","SlotIndex":0,"MeldPath":"12"}]}`
	var event Event
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatal(err)
	}
	if event.Mode != "roguelike" || event.Area != "Forest" || event.Level != "SomeRoom" ||
		event.Difficulty == nil || *event.Difficulty != 2.5 || event.RunStep == nil || *event.RunStep != 0 ||
		event.Commit != "e595535" || event.Platform != "windows" {
		t.Fatalf("event snapshot was decoded incorrectly: %+v", event)
	}
	if len(event.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(event.Items))
	}
	item := event.Items[0]
	if item.Level != 300 || item.SlotIndex == nil || *item.SlotIndex != 0 || item.MeldPath != "12" || item.Parent != "ExistingLongParentItemName" {
		t.Fatalf("melded item snapshot was decoded incorrectly: %+v", item)
	}
	var legacy Event
	if err := json.Unmarshal([]byte(`{"Type":"level_cleared","Cursemark":42,"Items":[{"Item":"sword","Level":5}]}`), &legacy); err != nil {
		t.Fatal(err)
	}
	if legacy.Difficulty != nil || legacy.RunStep != nil || legacy.Cursemark != 42 || legacy.Items[0].SlotIndex != nil {
		t.Fatalf("legacy payload should preserve its original fields and leave new values unknown: %+v", legacy)
	}
}

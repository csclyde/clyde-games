package models

import (
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
	"time"
)

func healthTime(s string) *time.Time {
	v, e := time.Parse(time.RFC3339Nano, s)
	if e != nil {
		panic(e)
	}
	return &v
}
func TestCrashHealthSeparatesLegacyTimeIdentityAndReturnedImpact(t *testing.T) {
	now := *healthTime("2026-11-16T12:00:00Z")
	from := now.Add(-7 * 24 * time.Hour)
	before := from.Add(-time.Nanosecond)
	d := func(id uint, project, key, pid, build, at string, resolved bool, closed *time.Time) CrashHealthRecord {
		return CrashHealthRecord{ID: id, Project: project, GroupKey: key, PID: pid, Build: build, Count: 999, OccurrenceVersion: 1, IngestedAt: healthTime(at), Resolved: resolved, ResolvedAt: closed, AccessViolation: true}
	}
	rows := []CrashHealthRecord{
		{ID: 1, Project: "cursemark", GroupKey: "a", PID: "lost legacy PID", Build: "not attributable", Count: 42, Resolved: true, IngestedAt: &now, AccessViolation: true},
		d(2, "cursemark", "a", "same-install", "old", "2026-11-10T00:00:00Z", true, healthTime("2026-11-11T00:00:00Z")),
		d(3, "cursemark", "a", "same-install", "new", "2026-11-12T00:00:00Z", false, nil), d(4, "cursemark", "a", "same-install", "new", "2026-11-13T00:00:00Z", false, nil), d(5, "cursemark", "a", " ", "new", "2026-11-14T00:00:00Z", false, nil),
		d(6, "cursemark", "b", "second-install", "old", "2026-11-10T00:00:00Z", true, healthTime("2026-11-12T00:00:00Z")), d(7, "cursemark", "b", "same-install", "new", "2026-11-15T00:00:00Z", false, nil),
		d(8, "necrovale", "a", "same-install", "new", "2026-11-13T00:00:00Z", false, nil),
		{ID: 9, Project: "cursemark", GroupKey: "legacy-only", Count: 100, IngestedAt: &now},
		{ID: 10, Project: "cursemark", GroupKey: "untimed", OccurrenceVersion: 1, PID: "not timed"},
		d(11, "cursemark", "future", "not received", "new", "2026-11-17T00:00:00Z", false, nil),
		{ID: 12, Project: "cursemark", GroupKey: "edge", OccurrenceVersion: 1, IngestedAt: &from, PID: "edge", Build: "edge"},
		{ID: 13, Project: "cursemark", GroupKey: "before", OccurrenceVersion: 1, IngestedAt: &before, PID: "before", Build: "before"},
		{ID: 14, Project: "cursemark", Count: 0},
	}
	original, _ := json.Marshal(rows)
	h := BuildCrashHealth(rows, "all", 7, now)
	if h.Reports != 8 || h.ReportingIDs != 4 || h.MissingPIDReports != 1 || h.LegacyRows != 3 || h.LegacyOccurrences != 142 || h.DetailedAllTime != 11 || h.UntimedDetailed != 2 {
		t.Fatalf("counts %+v", h)
	}
	if h.ReturnedOpenGroups != 2 || h.ReturnedReports != 4 || h.ReturnedReportingIDs != 1 {
		t.Fatalf("returned %+v", h)
	}
	if !h.FirstReceived.Equal(before) || !h.LastReceived.Equal(*healthTime("2026-11-15T00:00:00Z")) {
		t.Fatal("legacy/future receipt invented", h.FirstReceived, h.LastReceived)
	}
	sum := 0
	for _, d := range h.DaysData {
		sum += d.Reports
	}
	if sum != 8 || len(h.DaysData) != 8 {
		t.Fatal("UTC buckets", sum, len(h.DaysData))
	}
	for _, b := range h.Builds {
		if b.Build == "not attributable" {
			t.Fatal("legacy count attributed to build")
		}
		if b.Project == "cursemark" && b.Build == "new" && (b.Reports != 4 || b.ReportingIDs != 1 || b.ReturnedReports != 4 || b.Groups != 2 || b.MissingPIDReports != 1) {
			t.Fatalf("build %+v", b)
		}
	}
	for _, f := range h.Families {
		if f.Project == "cursemark" && (f.ReturnedOpenGroups != 2 || f.ReturnedReportingIDs != 1 || f.ReportingIDs != 2) {
			t.Fatalf("family lost dedup %+v", f)
		}
		if f.Project == "necrovale" && f.ReturnedOpen {
			t.Fatal("borrowed another project's resolution")
		}
	}
	encoded, _ := json.Marshal(h)
	if strings.Contains(string(encoded), "same-install") || strings.Contains(string(encoded), "lost legacy") {
		t.Fatal("PID evidence leaked in aggregates")
	}
	var contract map[string]interface{}
	json.Unmarshal(encoded, &contract)
	if contract["from"] == nil || contract["to"] == nil {
		t.Fatal("window contract missing", string(encoded))
	}
	after, _ := json.Marshal(rows)
	if string(original) != string(after) {
		t.Fatal("aggregation mutated original records")
	}
}
func TestCrashHealthDoesNotInferReturnFromUnknownLegacyResolution(t *testing.T) {
	now := *healthTime("2026-11-16T12:00:00Z")
	rows := []CrashHealthRecord{{ID: 1, Project: "cursemark", GroupKey: "a", Count: 100, Resolved: true}, {ID: 2, Project: "cursemark", GroupKey: "a", OccurrenceVersion: 1, IngestedAt: &now, PID: "one"}}
	h := BuildCrashHealth(rows, "cursemark", 30, now)
	if h.ReturnedOpenGroups != 0 || h.ReportingIDs != 1 {
		t.Fatalf("unknown history inferred %+v", h)
	}
	empty := BuildCrashHealth(nil, "new", 7, now)
	if !empty.Complete || empty.FirstReceived != nil || len(empty.Builds) != 0 || len(empty.DaysData) != 8 {
		t.Fatal("empty data", empty)
	}
	rows[0].ResolvedAt = &now
	h = BuildCrashHealth(rows, "cursemark", 7, now)
	if h.ReturnedOpenGroups != 0 {
		t.Fatal("equal timestamp proves no ordering")
	}
}
func TestCrashHealthQueryScopesAndExcludesSoftDeletedRecords(t *testing.T) {
	db, m := cutoffMock(t)
	now := time.Now().UTC()
	m.ExpectQuery("SELECT .*LOWER.* FROM `crashes` WHERE project = .*deleted_at.*ORDER BY id LIMIT").WithArgs("%access violation%", "necrovale").WillReturnRows(sqlmock.NewRows([]string{"id", "project", "group_key", "p_id", "build", "count", "occurrence_version", "ingested_at", "resolved", "resolved_at", "access_violation"}).AddRow(1, "necrovale", "a", "p_id", "build", 1, 1, now, false, nil, true))
	h, e := SelectCrashHealth(db, "necrovale", 7, now)
	if e != nil || h.Project != "necrovale" || h.Reports != 1 || h.ReportingIDs != 1 {
		t.Fatal(h, e)
	}
}

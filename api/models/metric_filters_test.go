package models

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
)

func TestMetricFilterCTEAppliesBeforeNestedBaselines(t *testing.T) {
	f := MetricFilters{Project: "cursemark", CreatedFrom: "2026-11-01T00:00:00-08:00", CreatedTo: "2026-11-02T08:00:00Z", Build: "build' OR 1=1 --", Environment: "production", EventType: "clear_level", Level: "LevelA"}
	for _, card := range MetricCatalog() {
		id := card.ID
		if !GameplayMetricForProject(id, f.Project) {
			continue
		}
		q, args, e := filteredMetricSQL(id, f, "isolated_fixture")
		if e != nil || len(args) != 7 {
			t.Fatalf("%d %v %+v", id, e, args)
		}
		if !strings.HasPrefix(q, "WITH events AS (SELECT * FROM `isolated_fixture`.`events` WHERE deleted_at IS NULL AND (project = ? OR project IS NULL OR TRIM(project) = '') AND created_at >= ? AND created_at < ? AND build = ? AND env = ? AND type = ? AND level = ?)") {
			t.Fatal(q)
		}
		if strings.Contains(q, f.Build) || strings.Contains(q, "analytics.events") {
			t.Fatal("filter was interpolated or escaped fixture", q)
		}
		if id == 102 && !strings.Contains(q, ", global_stats AS") {
			t.Fatal("baseline composition", q)
		}
		if id == 132 && !strings.Contains(q, ", level_attempts AS") {
			t.Fatal("commented CTE invalid", q)
		}
	}
	a, e := f.Normalize()
	if e != nil || a.CreatedFrom != "2026-11-01T08:00:00Z" {
		t.Fatal(a, e)
	}
	f.CreatedFrom = a.CreatedFrom
	b, _ := f.Normalize()
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	if string(x) != string(y) {
		t.Fatal("equivalent timezone instants make distinct cache keys")
	}
	for _, id := range []int{38, 39, 999} {
		if _, _, e := filteredMetricSQL(id, f, "fixture"); e == nil {
			t.Fatal("unreviewed/raw card allowed", id)
		}
	}
}
func TestMetricFiltersRejectAmbiguousAndInvalidScope(t *testing.T) {
	for _, f := range []MetricFilters{{Project: "all"}, {Project: "unknown"}, {Project: "cursemark", CreatedFrom: "2026-11-01"}, {Project: "cursemark", CreatedFrom: "2026-11-02T00:00:00Z", CreatedTo: "2026-11-01T00:00:00Z"}, {Project: "cursemark", Level: "bad\x00value"}, {Project: "cursemark", Build: strings.Repeat("a", 129)}} {
		if _, e := f.Normalize(); e == nil {
			t.Fatalf("accepted %+v", f)
		}
	}
	if _, _, e := filteredMetricSQL(104, MetricFilters{Project: "cursemark"}, "schema`escape"); e == nil {
		t.Fatal("schema injection")
	}
}
func TestFilteredMetricUsesConnectionSchemaAndBoundsRows(t *testing.T) {
	db, m := cutoffMock(t)
	old, stores := AnalyticsDB, gameMetricStores
	AnalyticsDB, gameMetricStores = db, []metricStore{{"fixture", cursemarkProject}}
	t.Cleanup(func() { AnalyticsDB, gameMetricStores = old, stores })
	m.ExpectQuery("WITH events AS .*`fixture`.`events`.*project = .*build =").WithArgs("cursemark", "specific").WillReturnRows(sqlmock.NewRows([]string{"level_clears"}).AddRow(int64(7)))
	r, e := RunFilteredMetric(context.Background(), 104, MetricFilters{Project: "cursemark", Build: "specific"})
	if e != nil || len(r.Rows) != 1 || r.Rows[0][0] != int64(7) || r.Truncated {
		t.Fatal(r, e)
	}
	rows := sqlmock.NewRows([]string{"item"})
	for i := 0; i < 1001; i++ {
		rows.AddRow("item")
	}
	m.ExpectQuery("WITH events AS").WithArgs("cursemark").WillReturnRows(rows)
	r, e = RunFilteredMetric(context.Background(), 121, MetricFilters{Project: "cursemark"})
	if e != nil || len(r.Rows) != 1000 || !r.Truncated {
		t.Fatal("bound", r, e)
	}
}

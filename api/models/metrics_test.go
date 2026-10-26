package models

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestMetricScope(t *testing.T) {
	for _, card := range MetricCatalog() {
		original, args, err := metricSQL(card.ID, "all")
		if err != nil || len(args) != 0 {
			t.Fatalf("metric %d: %v", card.ID, err)
		}
		scoped, args, err := metricSQL(card.ID, "cursemark' OR 1=1 --")
		if err != nil || len(args) != 1 || strings.Contains(scoped, "OR 1=1") || !strings.Contains(scoped, "WHERE project = ?") {
			t.Fatalf("metric %d is not parameterized", card.ID)
		}
		if strings.Count(strings.ToUpper(original), "WITH ") > 0 && strings.HasPrefix(strings.TrimSpace(original), "WITH ") && !strings.Contains(scoped, ") , ") {
			t.Fatalf("metric %d has invalid CTE composition", card.ID)
		}
	}
	if _, _, err := metricSQL(1, ""); err == nil {
		t.Fatal("unknown queries must be rejected")
	}
}

// Opt-in integration check against the local analytics database. Runs SELECTs
// only, and verifies every imported query for all, matching, and empty projects.
func TestMetricsLiveParity(t *testing.T) {
	if os.Getenv("METRICS_LIVE_TEST") != "1" {
		t.Skip("set METRICS_LIVE_TEST=1 to check local analytics")
	}
	var err error
	AnalyticsDB, err = GetDB("analytics")
	if err != nil {
		t.Fatal(err)
	}
	for _, card := range MetricCatalog() {
		t.Run(card.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			all, err := RunMetric(ctx, card.ID, "all")
			if err != nil {
				t.Fatal(err)
			}
			matching, err := RunMetric(ctx, card.ID, "cursemark")
			if err != nil {
				t.Fatal(err)
			}
			// Only events currently belong exclusively to Cursemark; feedback
			// and crash logs may include other projects.
			if card.ID != 38 && card.ID != 39 && !reflect.DeepEqual(canonicalRows(all.Rows), canonicalRows(matching.Rows)) {
				t.Fatal("project-scoped values differ from the original query")
			}
			empty, err := RunMetric(ctx, card.ID, "__metrics_nonexistent_project__")
			if err != nil {
				t.Fatal(err)
			}
			if card.Display != "scalar" && len(empty.Rows) != 0 {
				t.Fatal("project filter leaked rows")
			}
			t.Logf("%d rows, %d columns, truncated=%v", len(all.Rows), len(all.Columns), all.Truncated)
		})
	}
}

// Rows with tied ORDER BY keys have no guaranteed relative order.
func canonicalRows(rows [][]interface{}) []string {
	values := make([]string, len(rows))
	for i, row := range rows {
		data, _ := json.Marshal(row)
		values[i] = string(data)
	}
	sort.Strings(values)
	return values
}

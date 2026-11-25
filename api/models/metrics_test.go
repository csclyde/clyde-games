package models

import (
	"strings"
	"testing"
)

func TestMetricScope(t *testing.T) {
	for _, card := range MetricCatalog() {
		if !GameplayMetricAllowed(card.ID) {
			continue
		}
		project := metricProject(card)
		store := metricStore{schema: project + "_fixture", project: project}
		query, err := gameplaySQL(card.ID, store, []string{store.projectFilter()})
		if err != nil || !strings.Contains(query, store.sqlTable("events")) || !strings.Contains(query, store.sqlTable("event_items")) {
			t.Fatalf("metric %d escaped %s: %s (%v)", card.ID, project, query, err)
		}
		if strings.Contains(query, "WITH RECURSIVE") && !strings.HasPrefix(query, "WITH RECURSIVE events AS") {
			t.Fatalf("invalid recursive query %d", card.ID)
		}
	}
	for _, id := range []int{1, 38, 39} {
		if GameplayMetricAllowed(id) {
			t.Fatalf("unexpected gameplay metric %d", id)
		}
	}
}

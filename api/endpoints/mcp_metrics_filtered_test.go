package endpoints

import (
	"api.clyde.games/models"
	"context"
	"encoding/json"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFilteredMetricValidationBeforeDatabase(t *testing.T) {
	r, _ := mcpMockDB(t)
	for _, path := range []string{"metric-query/38?project=cursemark", "metric-query/104?project=necrovale", "metric-query/104?project=all", "metric-query/104?project=cursemark&sql=SELECT", "metric-query/104?project=cursemark&build=a&build=b", "metric-query/104?project=cursemark&created_from=2026-11-01"} {
		w := mcpRequest(r, "GET", path)
		if w.Code != 400 && w.Code != 404 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
}
func TestFilteredMetricCacheIsolationAndSingleFlight(t *testing.T) {
	r, _ := mcpMockDB(t)
	old := runFilteredMetric
	metricCache.Lock()
	previous := metricCache.entries
	metricCache.entries = map[string]*metricCacheEntry{}
	metricCache.Unlock()
	defer func() {
		runFilteredMetric = old
		metricCache.Lock()
		metricCache.entries = previous
		metricCache.Unlock()
	}()
	var calls atomic.Int32
	runFilteredMetric = func(ctx context.Context, id int, f models.MetricFilters) (*models.MetricResult, error) {
		calls.Add(1)
		time.Sleep(20 * time.Millisecond)
		return &models.MetricResult{Columns: []string{"build"}, Rows: [][]interface{}{{f.Build}}, UpdatedAt: time.Now().UTC()}, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := mcpRequest(r, "GET", "metric-query/104?project=cursemark&build=A")
			if w.Code != 200 {
				t.Errorf("%d %s", w.Code, w.Body)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("no single-flight", calls.Load())
	}
	w := mcpRequest(r, "GET", "metric-query/104?project=cursemark&build=B")
	var body map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 200 || calls.Load() != 2 || body["rows"].([]interface{})[0].([]interface{})[0] != "B" || body["complete"] != true {
		t.Fatal("cache leaked cohort", w.Code, w.Body, calls.Load())
	}
	unauth := httptest.NewRecorder()
	r.ServeHTTP(unauth, httptest.NewRequest("GET", "/internal/mcp/metric-query/104?project=cursemark", nil))
	if unauth.Code != 401 {
		t.Fatal("auth bypass", unauth.Code)
	}
}

func TestMCPCrashHealthRejectsAmbiguousScopeBeforeDatabase(t *testing.T) {
	r, _ := mcpMockDB(t)
	for _, path := range []string{"crash-health", "crash-health?project=cursemark&days=1", "crash-health?project=cursemark&days=7&days=90", "crash-health?project=cursemark&sql=SELECT", "crash-health?project=cur%00semark"} {
		w := mcpRequest(r, "GET", path)
		if w.Code != 400 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	for _, path := range []string{"crash-health?project=cursemark", "metric-query/104?project=cursemark"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/internal/mcp/"+path, nil))
		if w.Code != 401 {
			t.Fatalf("auth %s: %d", path, w.Code)
		}
	}
}

package main

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricAnalysisToolsAreReadOnlyAndForwardScopedFilters(t *testing.T) {
	var paths []string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Error("not read-only")
		}
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("u", 32) {
			t.Error("auth missing")
		}
		paths = append(paths, r.URL.Path)
		if r.URL.Path == "/internal/mcp/metric-query/104" {
			q := r.URL.Query()
			if q.Get("project") != "cursemark" || q.Get("build") != "release & hotfix" || q.Get("created_from") != "2026-11-01T00:00:00Z" {
				t.Error(q)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"complete": true})
	}))
	defer up.Close()
	b, _ := newBackend(up.URL, strings.Repeat("u", 32))
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	ss, e := makeServer(b).Connect(ctx, st, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer ss.Close()
	cs, e := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer cs.Close()
	listed, _ := cs.ListTools(ctx, nil)
	found := 0
	for _, tool := range listed.Tools {
		if tool.Name == "query_metric" || tool.Name == "get_crash_health" {
			found++
			if !tool.Annotations.ReadOnlyHint {
				t.Fatal(tool.Name)
			}
		}
	}
	if found != 2 {
		t.Fatal(found)
	}
	for _, tc := range []struct {
		name string
		args map[string]interface{}
	}{{"query_metric", map[string]interface{}{"id": 104, "project": "cursemark", "build": "release & hotfix", "created_from": "2026-11-01T00:00:00Z"}}, {"get_crash_health", map[string]interface{}{"project": "necrovale", "days": 7}}} {
		result, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if e != nil || result.IsError {
			t.Fatal(result, e)
		}
	}
	for _, args := range []map[string]interface{}{{"id": 104, "project": "all"}, {"id": 104, "project": "cursemark", "created_from": "yesterday"}, {"id": 104, "project": "cursemark", "sql": "SELECT"}} {
		result, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "query_metric", Arguments: args})
		if e == nil && !result.IsError {
			t.Fatal("invalid args dispatched")
		}
	}
	if len(paths) != 2 {
		t.Fatal(paths)
	}
}

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

func TestCrashWorkToolsAndNoIndividualInitiation(t *testing.T) {
	calls := []string{}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("u", 32) {
			t.Fatal("auth")
		}
		if r.URL.Path != "/internal/mcp/crash-investigations" && r.URL.Query().Get("project") != "cursemark" {
			t.Fatal("scope")
		}
		if r.Method == "POST" {
			var b map[string]any
			json.NewDecoder(r.Body).Decode(&b)
			if b["project"] != nil || b["id"] != nil {
				t.Fatal("arbitrary scope in payload")
			}
			if strings.HasSuffix(r.URL.Path, "/notes") && b["claim_token"] != "claim-token" {
				t.Fatal("missing claim")
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"unit_type":"crash_group","acquired":true}`))
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
	cs, e := mcp.NewClient(&mcp.Implementation{Name: "group-test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer cs.Close()
	tools, e := cs.ListTools(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	names := map[string]*mcp.Tool{}
	for _, v := range tools.Tools {
		names[v.Name] = v
	}
	for _, n := range []string{"get_crash_investigation", "claim_crash_investigation", "append_crash_investigation_note"} {
		if names[n] == nil {
			t.Fatal("missing tool", n)
		}
	}
	if !names["get_crash_investigation"].Annotations.ReadOnlyHint || names["claim_crash_investigation"].Annotations.ReadOnlyHint || !names["append_crash_investigation_note"].Annotations.IdempotentHint {
		t.Fatal("annotations")
	}
	for _, tc := range []struct {
		n string
		a map[string]any
	}{{"list_active_crashes", map[string]any{}}, {"get_crash_investigation", map[string]any{"id": 1, "project": "cursemark", "limit": 20}}, {"claim_crash_investigation", map[string]any{"id": 2, "project": "cursemark", "request_key": "group-run"}}, {"append_crash_investigation_note", map[string]any{"id": 1, "project": "cursemark", "request_key": "outcome-1", "claim_token": "claim-token", "note": "Local fix tested; unreleased.", "state": "ready_for_review", "release_state": "local", "complete": true}}} {
		r, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.n, Arguments: tc.a})
		if e != nil || r.IsError {
			t.Fatalf("%s %v %v", tc.n, r, e)
		}
	}
	before := len(calls)
	for _, tc := range []struct {
		n string
		a map[string]any
	}{{"append_report_note", map[string]any{"kind": "crash", "id": 1, "project": "cursemark", "request_key": "old-path", "note": "new per-report investigation"}}, {"claim_crash_investigation", map[string]any{"id": 0, "project": "cursemark", "request_key": "run-key"}}, {"get_crash_investigation", map[string]any{"id": 1, "project": "cursemark", "limit": 51}}, {"append_crash_investigation_note", map[string]any{"id": 1, "project": "cursemark", "request_key": "note-key", "claim_token": "", "note": "unclaimed"}}} {
		r, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.n, Arguments: tc.a})
		if e == nil && !r.IsError {
			t.Fatal("invalid workflow accepted", tc.n)
		}
	}
	if len(calls) != before {
		t.Fatal("per-report/invalid calls reached API")
	}
	if calls[0] != "GET /internal/mcp/crash-investigations" {
		t.Fatal("normal queue is per-report", calls)
	}
}

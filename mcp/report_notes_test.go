package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestReportNoteToolsScopesAndPayload(t *testing.T) {
	calls := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("project") != "cursemark" || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("u", 32) {
			t.Error("scope/auth")
		}
		if r.Method == "POST" {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["request_key"] != "repeat-123" || body["note"] != "Tested locally. Unreleased." || body["kind"] != nil || body["id"] != nil || body["project"] != nil {
				t.Errorf("payload %+v", body)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ok":true}`))
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
	cs, e := mcp.NewClient(&mcp.Implementation{Name: "notes-test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if e != nil {
		t.Fatal(e)
	}
	defer cs.Close()
	listed, _ := cs.ListTools(ctx, nil)
	found := 0
	for _, tool := range listed.Tools {
		switch tool.Name {
		case "get_report_notes", "get_report_investigation_status", "list_report_investigations":
			found++
			if !tool.Annotations.ReadOnlyHint {
				t.Fatal("read annotation")
			}
		case "append_report_note":
			found++
			if !tool.Annotations.IdempotentHint || tool.Annotations.ReadOnlyHint {
				t.Fatal("append annotation")
			}
		}
	}
	if found != 4 {
		t.Fatal(found)
	}
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"get_report_notes", map[string]any{"kind": "crash", "id": 3907, "project": "cursemark", "limit": 2, "after_id": 5}},
		{"append_report_note", map[string]any{"kind": "feedback", "id": 7886, "project": "cursemark", "request_key": "repeat-123", "note": "Tested locally. Unreleased.", "state": "fixed", "release_state": "local"}},
		{"get_report_investigation_status", map[string]any{"kind": "feedback", "ids": []int{7886}, "project": "cursemark"}},
		{"list_report_investigations", map[string]any{"kind": "feedback", "project": "cursemark"}},
	} {
		r, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if e != nil || r.IsError {
			t.Fatalf("%s %v %v", tc.name, r, e)
		}
	}
	for _, a := range []map[string]any{{"kind": "all", "id": 1, "project": "cursemark"}, {"kind": "crash", "id": 0, "project": "cursemark"}, {"kind": "crash", "id": 1, "project": "cursemark", "limit": 51}, {"kind": "crash", "id": 1}} {
		r, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_report_notes", Arguments: a})
		if e == nil && !r.IsError {
			t.Fatal("invalid accepted")
		}
	}
	if calls != 4 {
		t.Fatal("invalid arguments dispatched", calls)
	}
}

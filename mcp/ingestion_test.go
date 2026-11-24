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

func TestIngestionToolsAreNarrowAndPreserveWriteBodies(t *testing.T) {
	paths := []string{}
	bodies := []map[string]interface{}{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		var body map[string]interface{}
		if r.Method == "POST" {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !strings.HasSuffix(r.URL.Path, "/intake") {
				t.Error(err)
			}
		}
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"durable":true,"retry_needed":false}`))
	}))
	defer upstream.Close()
	b, _ := newBackend(upstream.URL, strings.Repeat("u", 32))
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	ss, err := makeServer(b).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	tests := []struct {
		name string
		args map[string]interface{}
		path string
	}{
		{"get_feedback_ingestion_status", map[string]interface{}{}, "GET /internal/mcp/feedback-ingestion"},
		{"list_discord_feedback_batch", map[string]interface{}{}, "GET /internal/mcp/discord/batch"},
		{"get_discord_feedback_message", map[string]interface{}{"id": "1406020405231353938"}, "GET /internal/mcp/discord/messages/1406020405231353938"},
		{"intake_discord_feedback", map[string]interface{}{"id": "1406020405231353938"}, "POST /internal/mcp/discord/messages/1406020405231353938/intake"},
		{"set_discord_feedback_disposition", map[string]interface{}{"id": "1406020405231353938", "disposition": "noise", "note": "greeting", "revision": 0}, "POST /internal/mcp/discord/messages/1406020405231353938/disposition"},
		{"triage_feedback", map[string]interface{}{"id": 12, "state": "reviewed", "note": "translated", "translated": "The game freezes."}, "POST /internal/mcp/feedback/12/triage"},
	}
	for i, tc := range tests {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %v", tc.name, res, err)
		}
		if paths[i] != tc.path {
			t.Fatalf("%s route %s", tc.name, paths[i])
		}
	}
	if bodies[5]["translated"] != "The game freezes." || bodies[4]["disposition"] != "noise" {
		t.Fatal("write body lost")
	}
	calls := len(paths)
	for _, tc := range []struct {
		name string
		args map[string]interface{}
	}{
		{"intake_discord_feedback", map[string]interface{}{"id": "../123"}},
		{"set_discord_feedback_disposition", map[string]interface{}{"id": "123", "disposition": "delete", "note": ""}},
		{"triage_feedback", map[string]interface{}{"id": 12, "state": "resolved", "note": ""}},
	} {
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err != nil {
			t.Fatal(err)
		}
		if !res.IsError {
			t.Fatalf("accepted invalid args: %+v", tc)
		}
	}
	if len(paths) != calls {
		t.Fatal("invalid args reached backend")
	}
}

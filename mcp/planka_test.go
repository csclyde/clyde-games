package main

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlankaToolGates(t *testing.T) {
	calls := 0
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/internal/mcp/planka/move" {
			t.Error("bad route")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"moved":true}`))
	}))
	defer up.Close()
	b, _ := newBackend(up.URL, strings.Repeat("a", 32))
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
	for _, confirmed := range []bool{false, true} {
		r, e := cs.CallTool(ctx, &mcp.CallToolParams{Name: "move_planka_ticket", Arguments: map[string]any{"id": "6", "board_id": "3", "list_id": "5", "owner_confirmed": confirmed}})
		if e != nil {
			t.Fatal(e)
		}
		if r.IsError == confirmed {
			t.Fatalf("unexpected result confirmed=%v: %v", confirmed, r)
		}
	}
	if calls != 1 {
		t.Fatalf("unauthorized dispatch %d", calls)
	}
}

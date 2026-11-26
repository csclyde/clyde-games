package main

import (
	"context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuth(t *testing.T) {
	token := strings.Repeat("x", 32)
	for _, tc := range []struct {
		name, auth, origin, path string
		want                     int
	}{
		{"missing", "", "", "/mcp", 401}, {"wrong", "Bearer no", "", "/mcp", 401}, {"browser", "Bearer " + token, "https://evil.example", "/mcp", 403}, {"path", "Bearer " + token, "", "/else", 404}, {"ok", "Bearer " + token, "", "/mcp", 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", tc.path, nil)
			r.Header.Set("Authorization", tc.auth)
			r.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }), token).ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d", w.Code)
			}
		})
	}
}
func TestBackendOrigin(t *testing.T) {
	for _, u := range []string{"http://evil.example", "http://127.0.0.1/path", "https://user:pass@example.org", "https://example.org?a=b", "file:///tmp/a"} {
		if _, err := newBackend(u, strings.Repeat("a", 32)); err == nil {
			t.Fatal("accepted", u)
		}
	}
}
func TestToolRoundTrip(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("u", 32) {
			t.Error("upstream auth missing")
		}
		if r.URL.Path != "/internal/mcp/feedback/7/resolve" || r.Method != "POST" {
			t.Errorf("wrong route %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"resolved":true,"savegames_soft_deleted":true}`))
	}))
	defer upstream.Close()
	b, err := newBackend(upstream.URL, strings.Repeat("u", 32))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	ss, err := makeServer(b).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	listed, err := cs.ListTools(ctx, nil)
	if err != nil || len(listed.Tools) != 46 {
		t.Fatalf("tools %v %v", listed, err)
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "resolve_feedback", Arguments: map[string]any{"id": 7}})
	if err != nil || res.IsError {
		t.Fatalf("call %v %v", res, err)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "resolve_feedback", Arguments: map[string]any{"id": -1}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || calls != 1 {
		t.Fatalf("invalid id dispatched: calls=%d result=%v", calls, res)
	}
}
func TestBackendRefusesRedirect(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "https://example.com", 302) }))
	defer up.Close()
	b, _ := newBackend(up.URL, strings.Repeat("t", 32))
	if _, _, err := b.call(context.Background(), "GET", "/projects"); err == nil {
		t.Fatal("redirect accepted")
	}
}

func TestHTTPTransport(t *testing.T) {
	token := strings.Repeat("c", 32)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[],"truncated":false,"limit":200}`))
	}))
	defer upstream.Close()
	b, _ := newBackend(upstream.URL, strings.Repeat("u", 32))
	s := makeServer(b)
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	endpoint := httptest.NewServer(protected(h, token))
	defer endpoint.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "http-test", Version: "1"}, nil)
	hc := &http.Client{Transport: bearerTransport{token: token}}
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: endpoint.URL + "/mcp", HTTPClient: hc}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_active_feedback", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("HTTP call %v %v", res, err)
	}
}

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}

// Owner-directed closure must be available without asserting that a fix was tested.
func TestOwnerDirectedResolution(t *testing.T) {
	calls := map[string]int{}
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("resolution must POST: %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("u", 32) {
			t.Error("missing upstream auth")
		}
		calls[r.URL.Path]++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"resolved":true}`))
	}))
	defer up.Close()
	b, _ := newBackend(up.URL, strings.Repeat("u", 32))
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	ss, err := makeServer(b).Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "owner-test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	init := cs.InitializeResult()
	if !strings.Contains(init.Instructions, "owner's explicit instruction") {
		t.Fatalf("owner closure policy missing: %+v", init)
	}
	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, tool := range listed.Tools {
		if tool.Name != "resolve_feedback" && tool.Name != "resolve_crash" {
			continue
		}
		count++
		if !strings.Contains(tool.Description, "explicit owner instruction") {
			t.Errorf("missing owner path for %s", tool.Name)
		}
		if tool.Annotations.ReadOnlyHint || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
			t.Errorf("incorrect mutation annotation for %s", tool.Name)
		}
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tool.Name, Arguments: map[string]any{"id": 7}})
		if err != nil || res.IsError {
			t.Fatalf("owner closure call %s: %v %v", tool.Name, res, err)
		}
	}
	if count != 2 || calls["/internal/mcp/feedback/7/resolve"] != 1 || calls["/internal/mcp/crashes/7/resolve"] != 1 {
		t.Fatalf("wrong resolution calls: %v", calls)
	}
}

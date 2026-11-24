package endpoints

import (
	"api.clyde.games/models"
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMCPAuthFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, token, auth string
		want              int
	}{
		{"unset", "", "Bearer ", 401},
		{"short", "abc", "Bearer abc", 401},
		{"missing", strings.Repeat("a", 32), "", 401},
		{"wrong", strings.Repeat("a", 32), "Bearer " + strings.Repeat("b", 32), 401},
		{"no scheme", strings.Repeat("a", 32), strings.Repeat("a", 32), 401},
		{"valid", strings.Repeat("a", 32), "Bearer " + strings.Repeat("a", 32), 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MCP_UPSTREAM_TOKEN", tc.token)
			r := gin.New()
			r.GET("/", MCPAuth, func(c *gin.Context) { c.Status(204) })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", tc.auth)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d", w.Code, tc.want)
			}
			if tc.want == 204 && w.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("missing no-store")
			}
		})
	}
}
func TestMCPRoutesRequireToken(t *testing.T) {
	t.Setenv("MCP_UPSTREAM_TOKEN", strings.Repeat("a", 32))
	r := gin.New()
	RegisterMCPRoutes(r)
	for _, route := range r.Routes() {
		url := strings.ReplaceAll(route.Path, ":id", "1")
		req := httptest.NewRequest(route.Method, url, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Errorf("%s %s got %d", route.Method, url, w.Code)
		}
	}
}
func TestMCPInvalidIDsDoNotQuery(t *testing.T) {
	token := strings.Repeat("a", 32)
	t.Setenv("MCP_UPSTREAM_TOKEN", token)
	r := gin.New()
	RegisterMCPRoutes(r)
	for _, route := range []string{"feedback/0", "feedback/all", "crashes/-1", "crashes/4294967296"} {
		for _, suffix := range []string{"", "/resolve"} {
			method := http.MethodGet
			if suffix != "" {
				method = http.MethodPost
			}
			req := httptest.NewRequest(method, "/internal/mcp/"+route+suffix, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != 400 {
				t.Errorf("%s got %d", route+suffix, w.Code)
			}
		}
	}
}
func TestMCPTextBoundPreservesUnicode(t *testing.T) {
	r := mcpReport{Message: strings.Repeat("🌱", 600), Stack: strings.Repeat("x", 17000)}
	mcpBoundReport(&r, false)
	if len([]rune(r.Message)) != 512 || len(r.Stack) != 512 || !r.TextTruncated {
		t.Fatalf("incorrect truncation: %d %d %v", len([]rune(r.Message)), len(r.Stack), r.TextTruncated)
	}
	r = mcpReport{Message: "short"}
	mcpBoundReport(&r, true)
	if r.TextTruncated {
		t.Fatal("short text marked truncated")
	}
}

func TestMCPRequestTimeoutBudget(t *testing.T) {
	token := strings.Repeat("a", 32)
	t.Setenv("MCP_UPSTREAM_TOKEN", token)
	for _, tc := range []struct {
		route, url string
		want       time.Duration
	}{
		{"/internal/mcp/metrics/:id", "/internal/mcp/metrics/1", 55 * time.Second},
		{"/internal/mcp/feedback/:id", "/internal/mcp/feedback/1", 15 * time.Second},
	} {
		r := gin.New()
		r.GET(tc.route, MCPAuth, func(c *gin.Context) {
			deadline, ok := c.Request.Context().Deadline()
			remaining := time.Until(deadline)
			if !ok || remaining > tc.want || remaining < tc.want-time.Second {
				t.Errorf("%s: remaining %v want %v", tc.route, remaining, tc.want)
			}
			c.Status(204)
		})
		req := httptest.NewRequest("GET", tc.url, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r.ServeHTTP(httptest.NewRecorder(), req)
	}
}

func TestCanceledMetricReturnsExplicitJSONError(t *testing.T) {
	id := strconv.Itoa(models.MetricCatalog()[0].ID)
	key := id + ":cursemark"
	// An unfinished cache entry prevents starting a database query. Cancellation
	// should return an explicit failure immediately, not an empty successful body.
	metricCache.Lock()
	metricCache.entries[key] = &metricCacheEntry{ready: make(chan struct{}), expires: time.Now().Add(time.Minute)}
	metricCache.Unlock()
	t.Cleanup(func() { metricCache.Lock(); delete(metricCache.entries, key); metricCache.Unlock() })
	r := gin.New()
	r.GET("/metrics/:id", GetMetric)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest("GET", "/metrics/"+id+"?project=cursemark", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusGatewayTimeout || !strings.Contains(w.Header().Get("Content-Type"), "application/json") || !strings.Contains(w.Body.String(), `"error"`) {
		t.Fatalf("canceled metric: %d %s", w.Code, w.Body)
	}
}

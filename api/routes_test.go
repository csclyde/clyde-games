package main

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminRoutesFailClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("ADMIN_API_TOKEN", "")
	t.Setenv("MCP_UPSTREAM_TOKEN", "")
	t.Setenv("PACKRAT_DEPLOY_TOKEN", "")
	r := gin.New()
	registerRoutes(r)
	public := map[string]bool{"POST /feedback": true, "POST /crash": true, "POST /event": true, "POST /savegame": true, "GET /packrat/version": true, "POST /words/analyze": true, "POST /words/stats": true, "POST /words/unknown": true, "POST /words/update": true}
	for _, route := range r.Routes() {
		if public[route.Method+" "+route.Path] {
			continue
		}
		path := strings.ReplaceAll(route.Path, ":id", "1")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(route.Method, path, nil))
		if w.Code != 401 && w.Code != 403 {
			t.Errorf("unprotected %s %s: %d", route.Method, path, w.Code)
		}
	}
	for _, p := range []string{"/resolvefeedback", "/resolvecrash"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 404 {
			t.Errorf("GET mutation still exists: %s", p)
		}
	}
}
func TestGameAndWebsiteSubmissionRoutesStayPublic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	var names []string
	r.Use(func(c *gin.Context) { names = c.HandlerNames(); c.AbortWithStatus(204) })
	registerRoutes(r)
	for _, path := range []string{"/feedback", "/crash", "/event", "/savegame", "/words/analyze", "/words/stats", "/words/unknown", "/words/update", "/packrat/version"} {
		method := "POST"
		if path == "/packrat/version" {
			method = "GET"
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != 204 {
			t.Fatal(path, w.Code)
		}
		for _, n := range names {
			if strings.Contains(n, "Auth") {
				t.Errorf("public %s gained auth: %s", path, n)
			}
		}
	}
}

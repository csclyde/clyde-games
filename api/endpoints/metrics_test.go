package endpoints

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMetricsAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, configured := range []string{"", "test-private-token"} {
		t.Setenv("PACKRAT_DEPLOY_TOKEN", configured)
		for _, supplied := range []string{"", "wrong", "test-private-token"} {
			router := gin.New()
			router.GET("/metrics", MetricsAuth, GetMetrics)
			router.GET("/metrics/:id", MetricsAuth, GetMetric)
			for _, path := range []string{"/metrics", "/metrics/999"} {
				request := httptest.NewRequest(http.MethodGet, path, nil)
				request.Header.Set("X-Packrat-Deploy-Token", supplied)
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				want := http.StatusForbidden
				if configured != "" && supplied == configured {
					want = http.StatusOK
					if path != "/metrics" {
						want = http.StatusNotFound
					}
				}
				if response.Code != want {
					t.Fatalf("%s got %d, want %d", path, response.Code, want)
				}
			}
		}
	}
}

func TestMetricsProjectAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/metrics", GetMetrics)
	router.GET("/metrics/:id", GetMetric)
	for _, project := range []string{"packrat", "pathprodigy", "all"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/metrics?project="+project, nil))
		if w.Code != 200 || w.Body.String() != "[]" {
			t.Fatalf("catalog %s: %d %s", project, w.Code, w.Body.String())
		}
		w = httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/metrics/88?project="+project, nil))
		if w.Code != 404 {
			t.Fatalf("metric %s: %d", project, w.Code)
		}
	}
	for _, tc := range []struct{ project string; count int }{
		{"", 19}, {"cursemark", 19}, {"necrovale", 9},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", "/metrics?project="+tc.project, nil))
		var cards []struct { Project string }
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &cards) != nil || len(cards) != tc.count {
			t.Fatalf("catalog %q: %d, %d cards", tc.project, w.Code, len(cards))
		}
		want := ""
		if tc.project == "necrovale" { want = "necrovale" }
		for _, card := range cards { if card.Project != want { t.Fatalf("cross-project catalog for %q", tc.project) } }
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/metrics/201?project=cursemark", nil))
	if w.Code != 404 { t.Fatalf("cross-project Necrovale query returned %d", w.Code) }
}

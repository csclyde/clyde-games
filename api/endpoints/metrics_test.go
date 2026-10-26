package endpoints

import (
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

package endpoints

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCrashHealthInvalidWindowsFailBeforeDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", GetCrashHealth)
	for _, q := range []string{"?days=0", "?days=365", "?days=no", "?project=", "?project=" + strings.Repeat("x", 65)} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/"+q, nil))
		if w.Code != 400 {
			t.Fatalf("%s %d", q, w.Code)
		}
	}
}
func TestCrashHealthUsesExistingAdminBoundary(t *testing.T) {
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("x", 32))
	r := gin.New()
	r.GET("/", AdminAuth, GetCrashHealth)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/?days=7", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
}

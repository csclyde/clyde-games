package endpoints

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAdminAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := strings.Repeat("a", 32)
	for _, tc := range []struct {
		name, secret, header, origin, site string
		want                               int
	}{
		{"unset", "", token, "", "", 401}, {"short", "a", "a", "", "", 401}, {"missing", token, "", "", "", 401}, {"wrong", token, strings.Repeat("b", 32), "", "", 401}, {"valid", token, token, "https://admin.clyde.games", "same-origin", 204}, {"cli", token, token, "", "", 204}, {"evil origin", token, token, "https://evil.example", "", 403}, {"null origin", token, token, "null", "", 403}, {"evil fetch", token, token, "", "cross-site", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ADMIN_API_TOKEN", tc.secret)
			r := gin.New()
			r.POST("/", AdminAuth, func(c *gin.Context) { c.Status(204) })
			q := httptest.NewRequest("POST", "/", nil)
			q.Header.Set("X-Admin-API-Token", tc.header)
			q.Header.Set("Origin", tc.origin)
			q.Header.Set("Sec-Fetch-Site", tc.site)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, q)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d", w.Code, tc.want)
			}
		})
	}
}

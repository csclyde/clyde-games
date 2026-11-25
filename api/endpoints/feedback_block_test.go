package endpoints

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func TestFeedbackBlockMySQLLifecycle(t *testing.T) {
	db := syncTestDB(t)
	if err := db.AutoMigrate(&models.BlockedUser{}, &models.Event{}, &models.EventItem{}, &models.Savegame{}); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	token := strings.Repeat("a", 32)
	t.Setenv("PORTAL_AUTH_MODE", "legacy")
	t.Setenv("ADMIN_API_TOKEN", token)
	r := gin.New()
	r.POST("/feedback", AddFeedback)
	r.PUT("/user/:id/feedback-block", AdminAuth, BlockFeedbackUser)
	r.DELETE("/user/:id/feedback-block", AdminAuth, UnblockFeedbackUser)
	r.GET("/users/feedback-blocked", AdminAuth, GetFeedbackBlockedUsers)
	r.GET("/user/:id", AdminAuth, GetUserHistory)
	r.POST("/event", AddEvent)
	r.POST("/savegame", AddSavegame)

	request := func(method, url, body string, admin bool) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if strings.HasPrefix(url, "/savegame") {
			req.Header.Set("Content-Type", "application/octet-stream")
		}
		if admin {
			req.Header.Set("X-Admin-API-Token", token)
		}
		r.ServeHTTP(w, req)
		return w
	}
	expect := func(w *httptest.ResponseRecorder, status int, bodyContains string) {
		t.Helper()
		if w.Code != status || (bodyContains != "" && !strings.Contains(w.Body.String(), bodyContains)) {
			t.Fatalf("expected %d including %q; got %d: %s", status, bodyContains, w.Code, w.Body)
		}
	}
	postFeedback := func(pid, game string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(models.Feedback{PID: pid, Project: game, Message: "Regression fixture", Rating: 3})
		return request("POST", "/feedback", string(body), false)
	}

	const pid = "player-fixture"
	const path = "/user/" + pid + "/feedback-block"
	first := postFeedback(pid, "cursemark")
	expect(first, http.StatusCreated, "")
	var prior models.Feedback
	if err := json.Unmarshal(first.Body.Bytes(), &prior); err != nil || prior.ID == 0 {
		t.Fatalf("missing success feedback ID: %v", err)
	}
	expect(request("PUT", path, "", false), 401, "")
	expect(request("PUT", path, "", true), 200, `"feedbackBlocked":true`)
	expect(request("PUT", path, "", true), 200, `"feedbackBlocked":true`)
	expect(request("PUT", "/user/"+strings.Repeat("p", 65)+"/feedback-block", "", true), 400, "")
	expect(request("GET", "/users/feedback-blocked", "", true), 200, pid)
	expect(request("GET", "/user/"+pid, "", true), 200, `"FeedbackBlocked": true`)
	for _, game := range []string{"cursemark", "necrovale"} {
		expect(postFeedback(pid, game), 403, `"code":"FEEDBACK_BLOCKED"`)
	}
	expect(postFeedback(" "+pid+" ", "necrovale"), 403, `"code":"FEEDBACK_BLOCKED"`)
	expect(postFeedback("unblocked-player", "necrovale"), 201, "")
	var count int64
	if err := db.Model(&models.Feedback{}).Where("p_id = ?", pid).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("expected only the single historical report, got %d: %v", count, err)
	}
	expect(request("POST", "/event", `{"PID":"player-fixture","Project":"cursemark","Type":"fixture"}`, false), 201, "")
	expect(request("POST", "/savegame?pid="+pid+"&project=cursemark", "save fixture", false), 201, "")
	expect(request("DELETE", path, "", false), 401, "")
	expect(request("DELETE", path, "", true), 200, `"feedbackBlocked":false`)
	expect(request("GET", "/users/feedback-blocked", "", true), 200, "[]")
	expect(postFeedback(pid, "cursemark"), 201, "")
}

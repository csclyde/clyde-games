package endpoints

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func TestIgnoreCrashGroupMySQL(t *testing.T) {
	db := syncTestDB(t)
	t.Setenv("PORTAL_AUTH_MODE", "legacy")
	token := strings.Repeat("a", 32)
	t.Setenv("ADMIN_API_TOKEN", token)
	r := gin.New()
	r.POST("/crash/ignore", AdminAuth, IgnoreCrash)
	post := func(id uint, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/crash/ignore?id=%d", id), nil)
		if authenticated {
			req.Header.Set("X-Admin-API-Token", token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	base := models.Crash{Project: "cursemark", Message: "fixture group", Stack: "failure"}
	mustAdd := func(crash models.Crash) *models.Crash {
		t.Helper()
		got, err := models.AddCrash(crash)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	first, second := mustAdd(base), mustAdd(base)
	other := base
	other.Project = "necrovale"
	separate := mustAdd(other)
	if w := post(first.ID, false); w.Code != http.StatusUnauthorized {
		t.Fatalf("ignore is not protected: %d %s", w.Code, w.Body)
	}
	if w := post(0, true); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid ignore ID: %d %s", w.Code, w.Body)
	}
	if w := post(separate.ID+500, true); w.Code != http.StatusNotFound {
		t.Fatalf("missing crash ID: %d %s", w.Code, w.Body)
	}
	for _, id := range []uint{first.ID, second.ID} {
		w := post(id, true)
		var got models.Crash
		if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &got) != nil ||
			!got.Resolved || got.ClosureDisposition != "ignored" || got.ResolvedAt == nil {
			t.Fatalf("ignore did not record group dismissal: %d %s", w.Code, w.Body)
		}
	}
	if _, err := models.ResolveCrashID(db, uint64(first.ID)); err != nil {
		t.Fatal(err)
	}
	next := mustAdd(base)
	var rows []models.Crash
	if err := db.Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || rows[0].ClosureDisposition != "ignored" || rows[1].ClosureDisposition != "ignored" ||
		rows[0].Stack != base.Stack || rows[1].Stack != base.Stack || rows[2].ID != separate.ID || rows[2].Resolved ||
		rows[3].ID != next.ID || rows[3].Resolved || rows[3].GroupKey != first.GroupKey {
		t.Fatalf("ignore affected unrelated, historical, or later reports: %+v", rows)
	}
}

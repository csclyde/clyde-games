package endpoints

import (
	"api.clyde.games/models"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestReviewSummaryCacheCoalescesAndExpires(t *testing.T) {
	cache := reviewSummaryCache{entries: make(map[string]*reviewSummaryEntry)}
	var calls int32
	fetch := func() (models.SteamReviewStats, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(20 * time.Millisecond)
		return models.SteamReviewStats{TotalReviews: 42}, nil
	}
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := cache.get("123", fetch)
			if e != nil || v.TotalReviews != 42 {
				t.Error("bad shared result")
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatalf("fetches=%d", calls)
	}
	cold := time.Since(start)
	start = time.Now()
	cache.get("123", fetch)
	warm := time.Since(start)
	if calls != 1 {
		t.Fatal("warm hit refetched")
	}
	t.Logf("simulated 20ms upstream: cold=%v warm=%v", cold, warm)
	cache.entries["123"].expires = time.Now().Add(-time.Second)
	cache.get("123", fetch)
	cache.get("456", fetch)
	if calls != 3 {
		t.Fatalf("expiry/key isolation fetches=%d", calls)
	}
}
func TestReviewSummaryCacheErrorRetries(t *testing.T) {
	cache := reviewSummaryCache{entries: make(map[string]*reviewSummaryEntry)}
	_, err := cache.get("1", func() (models.SteamReviewStats, error) { return models.SteamReviewStats{}, errors.New("offline") })
	if err == nil {
		t.Fatal("error lost")
	}
	cache.entries["1"].expires = time.Now().Add(-time.Second)
	_, err = cache.get("1", func() (models.SteamReviewStats, error) { return models.SteamReviewStats{}, nil })
	if err != nil {
		t.Fatal("error did not recover")
	}
}
func TestProjectMetadataSkipsAggregatesAndSteam(t *testing.T) {
	r, mock := mcpMockDB(t)
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("b", 32))
	r.GET("/projects", AdminAuth, GetProjects)
	mock.ExpectQuery("SELECT .* FROM `projects` ORDER BY name ASC, id ASC").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "steam_app_id"}).AddRow("test", "Test", "123"))
	mock.ExpectQuery("SELECT .* FROM `discord_sync_sources`").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	req := httptest.NewRequest("GET", "/projects?view=metadata", nil)
	req.Header.Set("X-Admin-API-Token", strings.Repeat("b", 32))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Test") {
		t.Fatalf("response %d %s", w.Code, w.Body.String())
	}
	// The mock permits only metadata queries, not analytics aggregation.
}

func TestProjectNavigationActiveCounts(t *testing.T) {
	r, mock := mcpMockDB(t)
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("b", 32))
	r.GET("/projects", AdminAuth, GetProjects)
	mock.ExpectQuery("SELECT .* FROM `projects` ORDER BY name ASC, id ASC").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow("cursemark", "Cursemark").AddRow("other", "Other"))
	mock.ExpectQuery("SELECT project, COUNT.* FROM `feedbacks` WHERE resolved != true AND parent_feedback_id IS NULL AND `feedbacks`.`deleted_at` IS NULL GROUP BY `project`").WillReturnRows(sqlmock.NewRows([]string{"project", "count"}).AddRow("Cursemark", 2))
	mock.ExpectQuery("SELECT project, COUNT.* FROM `crashes` WHERE resolved != true AND `crashes`.`deleted_at` IS NULL GROUP BY `project`").WillReturnRows(sqlmock.NewRows([]string{"project", "count"}).AddRow("cursemark", 3))
	mock.ExpectQuery("SELECT id, project FROM `feedbacks`.*LIMIT 5001").WillReturnRows(sqlmock.NewRows([]string{"id", "project"}).AddRow(72, "cursemark").AddRow(71, "Cursemark"))
	req := httptest.NewRequest("GET", "/projects?view=navigation", nil)
	req.Header.Set("X-Admin-API-Token", strings.Repeat("b", 32))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"openFeedback":2`) || !strings.Contains(w.Body.String(), `"openFeedback":0`) || !strings.Contains(w.Body.String(), `"openCrashes":3`) {
		t.Fatalf("response %d %s", w.Code, w.Body.String())
	}
}

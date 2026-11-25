package endpoints

import (
	"api.clyde.games/models"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	driver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func syncTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("FEEDBACK_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("requires disposable loopback DSN")
	}
	cfg, e := driver.ParseDSN(dsn)
	if e != nil {
		t.Fatal(e)
	}
	h, _, e := net.SplitHostPort(cfg.Addr)
	if e != nil || cfg.Net != "tcp" || cfg.DBName != "" || (h != "127.0.0.1" && h != "::1") {
		t.Fatal("database-less loopback required")
	}
	admin, e := sql.Open("mysql", dsn)
	if e != nil {
		t.Fatal(e)
	}
	name := fmt.Sprintf("sync_config_test_%d", time.Now().UnixNano())
	if _, e := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4"); e != nil {
		t.Fatal(e)
	}
	cfg.DBName = name
	cfg.ParseTime = true
	db, e := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	pool, _ := db.DB()
	old := models.AnalyticsDB
	models.AnalyticsDB = db
	t.Cleanup(func() {
		models.AnalyticsDB = old
		pool.Close()
		admin.Exec("DROP DATABASE `" + name + "`")
		admin.Close()
	})
	if e := db.AutoMigrate(&models.Project{}, &models.SyncState{}, &models.EventSetting{}, &models.Feedback{}, &models.FeedbackBlock{}, &models.DiscordPendingMessage{}, &models.DiscordSnapshotRevision{}, &models.DiscordSyncSource{}, &models.Crash{}, &models.CrashGroup{}); e != nil {
		t.Fatal(e)
	}
	if e := models.MigrateCrashInvestigations(db); e != nil {
		t.Fatal(e)
	}
	return db
}
func TestConfiguredSyncMigrationIsolationAndReenableBaseline(t *testing.T) {
	db := syncTestDB(t)
	db.Create(&models.Project{ID: "cursemark", Name: "Cursemark", SteamAppID: "3219180"})
	db.Create(&models.Project{ID: "necrovale", Name: "Necrovale", SteamAppID: "111"})
	db.Create(&models.Project{ID: "packrat", Name: "Packrat"})
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	db.Create(&models.SyncState{Name: "steam_reviews_cursemark", LastSuccessAt: old})
	for i := 0; i < 2; i++ {
		if e := models.MigrateFeedbackSyncConfiguration(); e != nil {
			t.Fatal(e)
		}
	}
	sources, e := models.SelectDiscordSyncSources("cursemark", true)
	if e != nil || len(sources) != 2 {
		t.Fatalf("sources %+v %v", sources, e)
	}
	s, e := models.GetSyncState("steam_reviews_cursemark")
	if e != nil || !s.LastSuccessAt.Equal(models.OwnerFeedbackBaseline) || s.BaselineAt == nil {
		t.Fatalf("baseline %+v %v", s, e)
	}
	_, _, from, _, e := runFeedbackSource(models.Project{ID: "cursemark"}, "steam_reviews", "", func(a, b time.Time) (int, int, error) { return 0, 0, errors.New("source offline") })
	if e == nil || !from.Equal(models.OwnerFeedbackBaseline) {
		t.Fatal("overlap crossed owner baseline")
	}
	no := false
	yes := true
	at := models.OwnerFeedbackBaseline.Add(time.Hour)
	if _, e := models.UpdateProjectWithSyncs("necrovale", "Named Necrovale", "111", &no, nil, at); e != nil {
		t.Fatal(e)
	}
	enabled, e := models.SelectProjectsWithSteam()
	if e != nil || len(enabled) != 1 || enabled[0].ID != "cursemark" {
		t.Fatalf("enabled %+v %v", enabled, e)
	}
	if _, e := models.UpdateProjectWithSyncs("necrovale", "Named Necrovale", "111", &yes, nil, at); e != nil {
		t.Fatal(e)
	}
	s, _ = models.GetSyncState("steam_reviews_necrovale")
	if s.BaselineAt == nil || !s.BaselineAt.Equal(at) {
		t.Fatal("reenable baseline missing")
	}
	for _, source := range sources {
		if source.ProjectID != "cursemark" {
			t.Fatal("ownership")
		}
	}
	if _, e := models.UpdateProjectWithSyncs("necrovale", "Cannot steal", "111", &yes, &sources, at); e == nil {
		t.Fatal("cross-project source theft accepted")
	}
	p, _ := models.SelectProject("necrovale")
	if p.Name != "Named Necrovale" {
		t.Fatal("failed update was not atomic")
	}
}
func TestForumRevisionCanonicalIdentityAndHandledHistory(t *testing.T) {
	db := syncTestDB(t)
	baseline := models.OwnerFeedbackBaseline
	db.Create(&models.Project{ID: "game", Name: "Game"})
	source := models.DiscordSyncSource{ID: "333", ProjectID: "game", GuildID: "111", Kind: "discord_forum", Name: "Forum", Enabled: true, BaselineAt: baseline}
	db.Create(&source)
	makeRaw := func(id, content string, at time.Time, handled, contextOnly bool) []byte {
		v := map[string]interface{}{"id": id, "channel_id": id, "guild_id": "111", "parent_id": "333", "scope_id": "333", "kind": "forum_thread", "timestamp": at, "context_timestamp": baseline.Add(time.Minute), "content": content, "thread_context": []interface{}{}, "handled": handled, "context_only": contextOnly, "feedback": map[string]interface{}{"PID": "author", "Message": content, "Project": "wrong"}}
		b, _ := json.Marshal(v)
		return b
	}
	first := makeRaw("101", "root", baseline.Add(time.Second), false, false)
	if changed, e := storeDiscordSnapshot(source, first, false); e != nil || !changed {
		t.Fatalf("store %v %v", changed, e)
	}
	if changed, e := storeDiscordSnapshot(source, first, false); e != nil || changed {
		t.Fatalf("replay %v %v", changed, e)
	}
	next := makeRaw("101", "edited root", baseline.Add(time.Second), false, false)
	if _, e := storeDiscordSnapshot(source, next, false); e != nil {
		t.Fatal(e)
	}
	var row models.DiscordPendingMessage
	db.First(&row, "id = ?", "101")
	if row.Revision != 2 || row.OriginalSnapshot != string(first) || !row.NeedsReview {
		t.Fatalf("revision %+v", row)
	}
	var snapshot discordSnapshot
	json.Unmarshal(first, &snapshot)
	for i := 0; i < 2; i++ {
		if _, e := discordIntake(snapshot); e != nil {
			t.Fatal(e)
		}
	}
	var count int64
	db.Model(&models.Feedback{}).Count(&count)
	if count != 1 {
		t.Fatal("reply/root duplicate")
	}
	var feedback models.Feedback
	db.First(&feedback)
	if feedback.Project != "game" {
		t.Fatal("project routing ignored")
	}
	db.Model(&feedback).Update("resolved", true)
	if _, e := discordIntake(snapshot); e != nil {
		t.Fatal(e)
	}
	db.First(&feedback)
	if !feedback.Resolved {
		t.Fatal("replay reopened resolved issue")
	}
	for _, raw := range [][]byte{makeRaw("202", "old root", baseline.Add(-time.Hour), true, true), makeRaw("203", "already marked", baseline.Add(time.Second), true, false)} {
		if _, e := storeDiscordSnapshot(source, raw, false); e != nil {
			t.Fatal(e)
		}
	}
	row = models.DiscordPendingMessage{}
	row = models.DiscordPendingMessage{}
	db.First(&row, "id = ?", "202")
	if row.Disposition != "handled" || row.FeedbackID != 0 {
		t.Fatal("history resurrected")
	}
	db.Model(&models.Feedback{}).Count(&count)
	if count != 1 {
		t.Fatal("handled material imported")
	}
	var revisions int64
	db.Model(&models.DiscordSnapshotRevision{}).Where("message_id = ?", "101").Count(&revisions)
	if revisions != 2 {
		t.Fatal("evidence revisions lost")
	}
}
func TestProjectCrashCutoffMigrationRejectBeforeStore(t *testing.T) {
	db := syncTestDB(t)
	for _, id := range []string{"alpha", "beta"} {
		db.Create(&models.Project{ID: id, Name: id})
	}
	db.Create(&models.EventSetting{Key: "oldest_crash_build", Value: "2026-10-01"})
	for _, p := range []string{"alpha", "beta"} {
		if _, e := models.AddCrash(models.Crash{Project: p, Build: "2026-10-02", Message: "existing", Stack: "frame"}); e != nil {
			t.Fatal(e)
		}
	}
	if e := models.MigrateProjectCrashCutoffs(); e != nil {
		t.Fatal(e)
	}
	for _, p := range []string{"alpha", "beta", ""} {
		s, e := models.GetCrashSettings(p)
		if e != nil || s.OldestBuild != "2026-10-01" {
			t.Fatalf("threshold %s %+v %v", p, s, e)
		}
	}
	if _, e := models.SaveOldestCrashBuild("2026-10-02", "alpha"); e != nil {
		t.Fatal(e)
	}
	if e := models.MigrateProjectCrashCutoffs(); e != nil {
		t.Fatal(e)
	}
	a, _ := models.GetCrashSettings("alpha")
	b, _ := models.GetCrashSettings("beta")
	if a.OldestBuild != "2026-10-02" || b.OldestBuild != "2026-10-01" {
		t.Fatal("migration overwrote project cutoff")
	}
	var before, after int64
	db.Model(&models.Crash{}).Count(&before)
	for _, c := range []models.Crash{{Project: "alpha", Build: "2026-10-01"}, {Project: "beta", Build: "2026-09-30"}, {Project: "", Build: "2026-09-30"}} {
		if _, e := models.AddCrash(c); !errors.Is(e, models.ErrBuildTooOld) {
			t.Fatalf("old build accepted %+v %v", c, e)
		}
	}
	db.Model(&models.Crash{}).Count(&after)
	if after != before {
		t.Fatal("rejected requests stored rows")
	}
	for _, c := range []models.Crash{{Project: "alpha", Build: "2026-10-02"}, {Project: "beta", Build: "2026-10-01"}, {Project: "new-project", Build: "2020-01-01"}} {
		if _, e := models.AddCrash(c); e != nil {
			t.Fatalf("boundary/default rejected %+v %v", c, e)
		}
	}
	db.Model(&models.Crash{}).Count(&after)
	if after != before+3 {
		t.Fatal("historical rows deleted or collapsed")
	}
	db.Where("`key` = ?", "oldest_build").First(&models.EventSetting{})
	legacy, _ := models.GetCrashSettings("")
	if legacy.OldestBuild != "2026-10-01" {
		t.Fatal("untagged compatibility changed")
	}
}

func TestDiscordConcurrentSnapshotAndRevisionDisposition(t *testing.T) {
	db := syncTestDB(t)
	baseline := models.OwnerFeedbackBaseline
	db.Create(&models.Project{ID: "game", Name: "Game"})
	source := models.DiscordSyncSource{ID: "333", ProjectID: "game", GuildID: "111", Kind: "discord_forum", Enabled: true, BaselineAt: baseline}
	db.Create(&source)
	raw := []byte(`{"id":"101","channel_id":"101","guild_id":"111","parent_id":"333","kind":"forum_thread","timestamp":"2026-11-16T05:58:22Z","content":"root","thread_context":[]}`)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := storeDiscordSnapshot(source, raw, false); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var row models.DiscordPendingMessage
	db.First(&row, "id = ?", "101")
	if row.Revision != 1 {
		t.Fatal("duplicate revision", row.Revision)
	}
	next := []byte(strings.Replace(string(raw), `"content":"root"`, `"content":"edited"`, 1))
	if _, e := storeDiscordSnapshot(source, next, false); e != nil {
		t.Fatal(e)
	}
	// Simulate a selected intake completing after newer player context arrived.
	if e := db.Model(&models.DiscordPendingMessage{}).Where("id = ?", "101").Updates(map[string]interface{}{"disposition": "imported", "feedback_id": 1, "needs_review": gorm.Expr("CASE WHEN revision = ? THEN false ELSE needs_review END", 1), "reviewed_revision": gorm.Expr("GREATEST(reviewed_revision, ?)", 1)}).Error; e != nil {
		t.Fatal(e)
	}
	row = models.DiscordPendingMessage{}
	db.First(&row, "id = ?", "101")
	if row.Revision != 2 || !row.NeedsReview || row.ReviewedRevision != 1 {
		t.Fatal("new context was acknowledged", row)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/messages/:id", mcpDiscordDisposition)
	request := func(revision int) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/messages/101", strings.NewReader(fmt.Sprintf(`{"disposition":"reviewed","revision":%d,"note":"context considered"}`, revision)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := request(1); w.Code != 409 {
		t.Fatalf("stale revision accepted %d %s", w.Code, w.Body)
	}
	for i := 0; i < 2; i++ {
		if w := request(2); w.Code != 200 {
			t.Fatalf("review replay %d %s", w.Code, w.Body)
		}
	}
	row = models.DiscordPendingMessage{}
	db.First(&row, "id = ?", "101")
	if row.NeedsReview || row.ReviewedRevision != 2 {
		t.Fatal("review not persisted", row)
	}
}

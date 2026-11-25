package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func TestFarmBuilderContractsMySQL(t *testing.T) {
	db := syncTestDB(t)
	check := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	check(db.Exec("ALTER TABLE crashes DROP INDEX idx_crashes_receipt_key, DROP COLUMN receipt_key").Error)
	check(db.Exec("INSERT INTO crashes (project,message,resolved) VALUES ('legacy-fixture','legacy history',TRUE)").Error)
	check(db.Create(&models.Project{ID: "farm-builder", Name: "farm-builder"}).Error)
	for i := 0; i < 2; i++ {
		check(models.MigrateAnalyticsDB())
	}
	check(models.MigrateReportNotes(db))
	project, err := models.SelectProject("farm-builder")
	check(err)
	if project == nil || project.Name != "Farm Builder" {
		t.Fatalf("project not registered before ingestion: %+v", project)
	}
	shared := db.Migrator().CurrentDatabase()
	cm, nv, farm := shared+"_cm", shared+"_nv", shared+"_farm"
	var migration bytes.Buffer
	t.Cleanup(func() {
		const prefix = "Game metrics archive database "
		for _, line := range strings.Split(migration.String(), "\n") {
			if strings.HasPrefix(line, prefix) {
				db.Exec("DROP DATABASE IF EXISTS " + strings.TrimPrefix(line, prefix))
			}
		}
		for _, schema := range []string{cm, nv, farm} {
			db.Exec("DROP DATABASE IF EXISTS " + schema)
		}
	})
	check(models.MigrateGameMetrics(db, cm, nv, false, &migration))
	restore, err := models.ConfigureGameMetrics(cm, nv)
	check(err)
	t.Cleanup(restore)
	if err := models.ProvisionFarmBuilderMetrics(db, cm); err == nil {
		t.Fatal("provisioning reused the Cursemark database")
	}
	for i := 0; i < 2; i++ {
		check(models.ProvisionFarmBuilderMetrics(db, farm))
	}
	restoreFarm, err := models.ConfigureFarmBuilderMetrics(farm)
	check(err)
	t.Cleanup(restoreFarm)

	r := gin.New()
	r.POST("/feedback", AddFeedback)
	r.POST("/crash", AddCrash)
	r.POST("/event", AddEvent)
	RegisterMCPRoutes(r)
	t.Setenv("MCP_UPSTREAM_TOKEN", strings.Repeat("m", 32))
	post := func(route, payload string, headers ...string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, route, strings.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		for _, value := range headers {
			req.Header.Add(models.CrashReportIDHeader, value)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	expect := func(w *httptest.ResponseRecorder, status int, dst interface{}) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("expected %d, got %d: %s", status, w.Code, w.Body.String())
		}
		if dst != nil {
			check(json.Unmarshal(w.Body.Bytes(), dst))
		}
	}
	var feedback, otherFeedback models.Feedback
	expect(post("/feedback", `{"Project":"farm-builder","PID":"farm-install","Message":"Farm feedback fixture","Rating":5,"Category":"general","Env":"prod","Platform":"windows","Build":"0.1.0"}`), 201, &feedback)
	expect(post("/feedback", `{"Project":"cursemark","PID":"farm-install","Message":"Other feedback fixture"}`), 201, &otherFeedback)
	if feedback.ID == 0 || feedback.Project != "farm-builder" {
		t.Fatalf("incorrect feedback attribution: %+v", feedback)
	}
	assertConcurrentReceipts := func(send func() *httptest.ResponseRecorder) uint {
		t.Helper()
		var writers sync.WaitGroup
		const attempts = 6
		results := make(chan *httptest.ResponseRecorder, attempts)
		for i := 0; i < attempts; i++ {
			writers.Add(1)
			go func() {
				defer writers.Done()
				results <- send()
			}()
		}
		writers.Wait()
		close(results)
		var id uint
		for result := range results {
			var receipt struct{ ID uint }
			expect(result, 201, &receipt)
			if receipt.ID == 0 || (id != 0 && id != receipt.ID) {
				t.Fatalf("concurrent retry changed receipt %d to %d", id, receipt.ID)
			}
			id = receipt.ID
		}
		return id
	}
	const crash = `{"Project":"farm-builder","PID":"farm-install","Message":"Recovered Godot fixture","Stack":"Godot script stack","Env":"prod","Platform":"windows","Build":"0.1.0"}`
	const reportID = "0123456789abcdef0123456789abcdef"
	crashID := assertConcurrentReceipts(func() *httptest.ResponseRecorder { return post("/crash", crash, reportID) })
	var replayCrash, another, legacyA, legacyB models.Crash
	expect(post("/crash", strings.Replace(crash, "Recovered Godot fixture", "Changed retry", 1), reportID), 201, &replayCrash)
	expect(post("/crash", strings.Replace(crash, "farm-install", "another-install", 1), reportID), 201, &another)
	if replayCrash.ID != crashID || replayCrash.Count != 1 || another.ID == crashID || replayCrash.Message != "Recovered Godot fixture" {
		t.Fatalf("incorrect crash replay: %+v / %+v", replayCrash, another)
	}
	legacy := strings.Replace(crash, "farm-builder", "cursemark", 1)
	expect(post("/crash", legacy, reportID), 201, &legacyA)
	expect(post("/crash", legacy, reportID), 201, &legacyB)
	if legacyA.ID == legacyB.ID {
		t.Fatal("old clients were deduplicated")
	}
	for _, value := range []string{"", "invalid", strings.ToUpper(reportID)} {
		expect(post("/crash", crash, value), 400, nil)
	}
	expect(post("/crash", crash, reportID, reportID), 400, nil)
	expect(post("/crash", strings.Replace(crash, "farm-install", "", 1), reportID), 400, nil)

	const season = `{"Project":"farm-builder","Type":"season_completed","SID":"abcdef0123456789abcdef0123456789","PID":"farm-install","Build":"0.1.0","Env":"production","Platform":"windows","RunStep":1,"Level":"spring","Gold":11,"Items":[{"Item":"Garden-Square","Slot":"piece","Level":2},{"Item":"garden-square","Slot":"piece","Level":1},{"Item":"Crop+L","Slot":"feature","Level":3}]}`
	eventID := assertConcurrentReceipts(func() *httptest.ResponseRecorder { return post("/event", season) })
	var replay, next models.Event
	expect(post("/event", strings.Replace(season, `"Gold":11`, `"Gold":99`, 1)), 201, &replay)
	expect(post("/event", strings.Replace(season, `"RunStep":1`, `"RunStep":2`, 1)), 201, &next)
	if replay.ID != eventID || replay.Gold == nil || *replay.Gold != 11 || len(replay.Items) != 3 || next.ID == eventID {
		t.Fatalf("incorrect season replay: %+v / %+v", replay, next)
	}
	expect(post("/event", strings.Replace(season, `"Gold":11`, `"Gold":-1`, 1)), 400, nil)
	expect(post("/event", strings.Replace(season, `"Level":"spring"`, `"Level":"unknown"`, 1)), 400, nil)
	expect(post("/event", `{"Project":"cursemark","Type":"clear_level","Level":"Other game","Resources":{"Essence":7}}`), 201, nil)
	expect(post("/event", `{"Project":"necrovale","Type":"player_died","Level":"TempleHope","Gold":9}`), 201, nil)
	wantRows := 2
	if file := os.Getenv("FARM_BUILDER_GODOT_FIXTURE"); file != "" {
		data, err := os.ReadFile(file)
		check(err)
		var receipt models.Event
		expect(post("/event", string(data)), 201, &receipt)
		if receipt.Project != "farm-builder" || receipt.Gold == nil || *receipt.Gold != 11 ||
			len(receipt.Items) != 3 || receipt.Items[0].Item != "Garden-Square" || receipt.Env != "development" {
			t.Fatalf("actual Godot capture changed during ingestion: %+v", receipt)
		}
		wantRows++
		t.Log("Actual Godot season fixture accepted by POST /event with no extra header")
	}
	for _, schema := range []string{shared, cm, nv, farm} {
		var count int
		check(db.Raw("SELECT COUNT(*) FROM "+schema+".events WHERE project = ?", "farm-builder").Scan(&count).Error)
		wanted := 0
		if schema == farm {
			wanted = wantRows
		}
		if count != wanted {
			t.Fatalf("%s has %d Farm Builder rows; expected %d", schema, count, wanted)
		}
	}
	var retained int64
	check(db.Model(&models.Crash{}).Where("message = ?", "legacy history").Count(&retained).Error)
	if retained != 1 {
		t.Fatal("additive migration lost legacy crash")
	}

	ctx := context.Background()
	for id, want := range map[int]string{301: "2", 302: "22"} {
		result, err := models.RunMetric(ctx, id, "farm-builder")
		check(err)
		if len(result.Rows) != 1 || fmt.Sprint(result.Rows[0][0]) != want {
			t.Fatalf("metric %d double counted: %+v", id, result)
		}
	}
	board, err := models.RunMetric(ctx, 303, "farm-builder")
	check(err)
	rows := fmt.Sprint(board.Rows)
	if len(board.Rows) != 3 || !strings.Contains(rows, "Garden-Square 4 2") ||
		!strings.Contains(rows, "garden-square 2 2") || !strings.Contains(rows, "Crop+L 6 2") {
		t.Fatalf("board identifiers or quantities changed: %+v", board.Rows)
	}
	exerciseMCPReports(t, r, func(call func(string, string, bool, string, string)) {
		call("list_projects", `{}`, false, "Farm Builder", "")
		call("list_active_feedback", `{"project":"farm-builder"}`, false, "Farm feedback fixture", "Other feedback fixture")
		call("list_active_feedback", `{}`, false, "Other feedback fixture", "")
		call("list_active_crashes", `{"project":"farm-builder"}`, false, "farm-builder", "cursemark")
		call("get_feedback", fmt.Sprintf(`{"id":%d,"project":"farm-builder"}`, feedback.ID), false, "Farm feedback fixture", "")
		call("get_feedback", fmt.Sprintf(`{"id":%d,"project":"farm-builder"}`, otherFeedback.ID), true, "", "")
		call("get_crash", fmt.Sprintf(`{"id":%d,"project":"farm-builder"}`, crashID), false, "Recovered Godot fixture", "Changed retry")
		call("get_crash", fmt.Sprintf(`{"id":%d,"project":"farm-builder"}`, legacyA.ID), true, "", "")
		call("list_metrics", `{"project":"farm-builder"}`, false, "Board Items", "Journey Completions")
		call("query_metric", `{"id":302,"project":"farm-builder","build":"0.1.0"}`, false, `"rows":[[22]]`, "")
		call("get_metric", `{"id":303,"project":"farm-builder"}`, false, "Crop+L", "")
		call("get_metric", `{"id":303,"project":"cursemark"}`, true, "", "")
	})
	restoreFarm()
	expect(post("/event", season), 503, nil)
}

package endpoints

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func TestGameMetricsCutoverAndRollback(t *testing.T) {
	db := syncTestDB(t)
	if err := db.AutoMigrate(&models.BlockedUser{}, &models.Savegame{}, &models.Event{}, &models.EventItem{}); err != nil {
		t.Fatal(err)
	}
	shared := db.Migrator().CurrentDatabase()
	cursemark, necrovale := shared+"_cm", shared+"_nv"
	t.Cleanup(func() {
		db.Exec("DROP DATABASE IF EXISTS " + cursemark)
		db.Exec("DROP DATABASE IF EXISTS " + necrovale)
	})

	r := gin.New()
	r.POST("/event", AddEvent)
	r.GET("/metrics", GetMetrics)
	r.GET("/metrics/:id", GetMetric)
	RegisterMCPRoutes(r)
	t.Setenv("MCP_UPSTREAM_TOKEN", strings.Repeat("m", 32))
	post := func(payload string) (*models.Event, error) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/event", strings.NewReader(payload)))
		if w.Code != 201 {
			return nil, fmt.Errorf("POST /event: %d %s", w.Code, w.Body)
		}
		var e models.Event
		return &e, json.Unmarshal(w.Body.Bytes(), &e)
	}
	mustPost := func(payload string) *models.Event {
		t.Helper()
		event, err := post(payload)
		if err != nil {
			t.Fatal(err)
		}
		return event
	}
	curse := mustPost(`{"Project":"Cursemark","Type":"clear_level","Level":"CurseOnly","Env":"prod","Build":"2026-11-23","Resources":{"Essence":17},"Items":[{"Item":"CurseSword"}]}`)
	legacy := mustPost(`{"Project":"Necrovale","Type":"player_died","Level":"NecroOnly","Mode":"journey","Env":"prod","Build":"2026-11-24","Items":[{"Item":"NecroSword"}]}`)
	projectless := mustPost(`{"Type":"clear_level","Level":"ProjectlessCurse"}`)
	other := mustPost(`{"Project":"other","Type":"visited","Level":"SharedOnly","Items":[{"Item":"OtherItem"}]}`)

	// A batch of legacy writes admitted immediately before the drain must be
	// present after the atomic switch, including children.
	var writers sync.WaitGroup
	for i := 0; i < 8; i++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			if _, err := post(`{"Project":"necrovale","Type":"player_died","Level":"DuringDrain","Build":"drain","Items":[{"Item":"Item"}]}`); err != nil {
				t.Error(err)
			}
		}()
	}
	writers.Wait()
	var log bytes.Buffer
	if err := models.MigrateGameMetrics(db, cursemark, necrovale, false, &log); err != nil {
		t.Fatalf("split: %v\n%s", err, log.String())
	}
	restore, err := models.ConfigureGameMetrics(cursemark, necrovale)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)

	count := func(table, filter string, expected int) {
		t.Helper()
		var n int
		if err := db.Raw("SELECT COUNT(*) FROM " + table + " WHERE " + filter).Scan(&n).Error; err != nil || n != expected {
			t.Fatalf("%s %s: got %d, want %d (%v)", table, filter, n, expected, err)
		}
	}
	count("events", "1", 1)
	count("event_items", "1", 1)
	count(cursemark+".events", "1", 2)
	count(cursemark+".event_items", "item = 'CurseSword'", 1)
	count(necrovale+".events", "1", 9)
	count(necrovale+".event_items", "1", 9)
	count(necrovale+".events", fmt.Sprintf("id = %d AND hopestones IS NULL AND gold IS NULL", legacy.ID), 1)
	count(cursemark+".events", fmt.Sprintf("id = %d AND resource_essence = 17", curse.ID), 1)
	count(cursemark+".events", fmt.Sprintf("id = %d", projectless.ID), 1)
	if err := models.MigrateGameMetrics(db, cursemark, necrovale, false, io.Discard); err == nil {
		t.Fatal("a second split must refuse to overwrite active tables")
	}

	next := mustPost(`{"Project":"necrovale","PID":"pending-player","Env":"prod","Build":"2026-11-25","Type":"complete_run","Level":"TempleHope","Mode":"journey","Hopestones":0,"Gold":25.5,"LifetimeClears":3,"LifetimeDeaths":9,"ClearHistoryComplete":false,"DeathHistoryComplete":true,"Items":[{"Item":"NextSword","SlotIndex":0,"MeldPath":"1"}]}`)
	mustPost(`{"Project":"necrovale","PID":"pending-player","Env":"prod","Build":"2026-11-25","Type":"player_died","Level":"TempleHope","Mode":"journey","Hopestones":0,"Gold":0,"LifetimeClears":3,"LifetimeDeaths":10,"ClearHistoryComplete":false,"DeathHistoryComplete":true}`)
	oldClient := mustPost(`{"Project":"cursemark","PID":"pending-player","Type":"clear_level","Level":"NewCurseOnly","Env":"prod","Build":"2026-11-25","Items":[{"Item":"NewCurseSword"}]}`)
	if next.ID <= other.ID || oldClient.ID <= next.ID {
		t.Fatalf("IDs were reused: %d %d %d", other.ID, next.ID, oldClient.ID)
	}
	count(necrovale+".events", fmt.Sprintf("id = %d AND hopestones = 0 AND gold = 25.5 AND lifetime_clears = 3 AND clear_history_complete = 0", next.ID), 1)
	history, err := models.SelectUserHistory("pending-player")
	if err != nil || len(history.Events) != 3 || len(history.Events[0].Items) != 1 || history.Events[0].Items[0].Item != "NewCurseSword" || len(history.Events[2].Items) != 1 {
		t.Fatalf("user event/item history lost cross-schema linkage: %+v, %v", history, err)
	}
	if n, err := models.DeleteEventsByBuild("drain", "necrovale"); err != nil || n != 8 {
		t.Fatalf("project build deletion: %d %v", n, err)
	}
	count(necrovale+".event_items", "item = 'Item'", 0)
	ctx := context.Background()
	for _, card := range models.MetricCatalog() {
		p := card.Project
		if p == "" {
			p = "cursemark"
		}
		if _, err := models.RunMetric(ctx, card.ID, p); err != nil {
			t.Errorf("dashboard %s metric %d: %v", p, card.ID, err)
		}
	}
	economy, err := models.RunMetric(ctx, 210, "necrovale")
	if err != nil || len(economy.Rows) != 3 {
		t.Fatalf("balance snapshots: %+v %v", economy, err)
	}
	counters, err := models.RunMetric(ctx, 211, "necrovale")
	if err != nil || len(counters.Rows) != 2 || fmt.Sprint(counters.Rows[0]) != "[journey Clears Partial history 1 3.0000 3]" ||
		!strings.HasPrefix(fmt.Sprint(counters.Rows[1]), "[journey Deaths Full history 1 10") {
		t.Fatalf("lifetime snapshots were summed or lost their coverage: %+v %v", counters, err)
	}
	mark := mustPost(`{"Project":"necrovale","PID":"blocked-fixture","Type":"player_died","Items":[{"Item":"BlockedItem"}]}`)
	if err := models.BlockUser("blocked-fixture"); err != nil {
		t.Fatal(err)
	}
	count(necrovale+".events", fmt.Sprintf("id = %d", mark.ID), 0)
	count(necrovale+".event_items", "item = 'BlockedItem'", 0)

	request := func(path string, status int, substr string) {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+strings.Repeat("m", 32))
		r.ServeHTTP(w, req)
		if w.Code != status || !strings.Contains(w.Body.String(), substr) {
			t.Fatalf("%s: got %d %s", path, w.Code, w.Body)
		}
	}
	request("/internal/mcp/metrics", http.StatusBadRequest, "explicitly")
	request("/internal/mcp/metrics?project=necrovale", http.StatusOK, "Balances by Build and Mode")
	request("/internal/mcp/metric-query/83?project=cursemark", http.StatusOK, "NewCurseOnly")
	request("/internal/mcp/metric-query/201?project=necrovale", http.StatusOK, `"rows":[[2]]`)
	request("/internal/mcp/metric-query/201?project=cursemark", http.StatusNotFound, "selected game")
	exerciseGameMetricsMCP(t, r)

	restore()
	if err := models.MigrateGameMetrics(db, cursemark, necrovale, true, &log); err != nil {
		t.Fatalf("rollback: %v\n%s", err, log.String())
	}
	count("events", "1", 7)
	count("event_items", "1", 5)
	count("events", fmt.Sprintf("id = %d AND gold = 25.5 AND lifetime_deaths = 9", next.ID), 1)
	count("events", "build = 'drain' OR p_id = 'blocked-fixture'", 0)
	if _, err := models.ConfigureGameMetrics(cursemark, necrovale); err == nil {
		t.Fatal("split configuration accepted a rolled-back database")
	}
	if _, err := models.RunFilteredMetric(ctx, 201, models.MetricFilters{Project: "necrovale"}); err == nil {
		t.Fatal("unavailable store silently fell back to shared metrics")
	}
	if err := models.MigrateGameMetrics(db, cursemark, necrovale, false, &log); err != nil {
		t.Fatalf("second split: %v\n%s", err, log.String())
	}
	count(necrovale+".events", fmt.Sprintf("id = %d AND gold = 25.5 AND lifetime_deaths = 9", next.ID), 1)
}

func exerciseGameMetricsMCP(t *testing.T, router http.Handler) {
	t.Helper()
	binary := os.Getenv("GAME_METRICS_MCP_BINARY")
	if binary == "" {
		t.Log("Set GAME_METRICS_MCP_BINARY to exercise the MCP HTTP entry point")
		return
	}
	upstream := httptest.NewServer(router)
	defer upstream.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	clientToken := strings.Repeat("c", 32)
	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(), "MCP_API_URL="+upstream.URL, "MCP_LISTEN="+address,
		"MCP_CLIENT_TOKEN="+clientToken, "MCP_UPSTREAM_TOKEN="+strings.Repeat("m", 32))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("MCP HTTP process did not start")
		}
		time.Sleep(50 * time.Millisecond)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	rpc := func(body string, status int) string {
		t.Helper()
		req, _ := http.NewRequest("POST", "http://"+address+"/mcp", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+clientToken)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("MCP-Protocol-Version", "2025-06-18")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != status {
			t.Fatalf("MCP HTTP %d, expected %d: %s", resp.StatusCode, status, raw)
		}
		return string(raw)
	}
	rpc(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"split-test","version":"1"}}}`, 200)
	rpc(`{"jsonrpc":"2.0","method":"notifications/initialized"}`, 202)
	call := func(tool, args string, wantError bool, want, absent string) {
		t.Helper()
		out := rpc(fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":%s}}`, tool, args), 200)
		var result struct {
			Error  json.RawMessage
			Result struct {
				IsError bool
				Content []struct{ Text string }
			}
		}
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatal(err, out)
		}
		isError := result.Result.IsError || len(result.Error) > 0
		if wantError {
			if !isError {
				t.Fatalf("%s unexpectedly succeeded: %s", tool, out)
			}
			return
		}
		if isError || len(result.Result.Content) != 1 || !strings.Contains(result.Result.Content[0].Text, want) ||
			(absent != "" && strings.Contains(result.Result.Content[0].Text, absent)) {
			t.Fatalf("%s %s: %s", tool, args, out)
		}
	}
	call("list_metrics", `{"project":"necrovale"}`, false, "Balances by Build and Mode", "Most played level")
	call("list_metrics", `{"project":"cursemark"}`, false, "Most played level", "Balances by Build and Mode")
	call("get_metric", `{"project":"cursemark","id":83}`, false, "CurseOnly", "NecroOnly")
	call("get_metric", `{"project":"necrovale","id":210}`, false, "average_hopestones", "CurseOnly")
	call("query_metric", `{"project":"necrovale","id":201,"level":"NecroOnly"}`, false, `"rows":[[1]]`, "")
	call("query_metric", `{"project":"cursemark","id":83,"level":"CurseOnly"}`, false, "CurseOnly", "NecroOnly")
	call("query_metric", `{"project":"cursemark","id":201}`, true, "", "")
	call("list_metrics", `{}`, true, "", "")
	call("get_metric", `{"project":"other","id":83}`, true, "", "")
}

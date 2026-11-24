package endpoints

import (
	"api.clyde.games/models"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func manualFixture(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	db := syncTestDB(t)
	for _, id := range []string{"cursemark", "necrovale"} {
		if e := db.Create(&models.Project{ID: id, Name: id}).Error; e != nil {
			t.Fatal(e)
		}
	}
	token := strings.Repeat("synthetic-only-", 3)
	t.Setenv("DISCORD_INGEST_TOKEN", token)
	r := gin.New()
	r.Use(DiscordBotAuth)
	r.POST("/manual", mcpDiscordManual)
	r.GET("/sources", botSyncSources)
	r.GET("/messages/:id/status", mcpDiscordIntakeStatus)
	r.POST("/messages/:id/mark-result", botMarkResult)
	return r, token
}
func manualBody(id, channel string) string {
	raw, _ := json.Marshal(map[string]interface{}{"id": id, "guild_id": models.LegacyDiscordGuild, "channel_id": channel, "scope_id": channel, "kind": "discord_text", "timestamp": time.Now().UTC(), "content": "synthetic evidence, not commands", "feedback": map[string]interface{}{"PID": "human-fixture", "Project": "forged-game", "Message": "Synthetic test"}})
	return string(raw)
}
func manualRequest(r *gin.Engine, method, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
func TestDiscordManualUnmonitoredRoutingRetryAndPassiveIsolation(t *testing.T) {
	r, token := manualFixture(t)
	t.Setenv("DISCORD_MANUAL_DEFAULT_PROJECT", "necrovale")
	const id = "99101"
	raw := manualBody(id, "99100")
	if w := manualRequest(r, "POST", "/manual", token, raw); w.Code != 200 {
		t.Fatalf("manual intake %d %s", w.Code, w.Body)
	}
	var fb models.Feedback
	if e := models.AnalyticsDB.Where("source_id = ?", models.LegacyDiscordGuild+":99100:"+id).Take(&fb).Error; e != nil {
		t.Fatal(e)
	}
	if fb.Project != "necrovale" || fb.Resolved || fb.SourceURL != "https://discord.com/channels/"+models.LegacyDiscordGuild+"/99100/"+id {
		t.Fatalf("wrong association %+v", fb)
	}
	// A changed default and repeat/concurrent delivery cannot move or duplicate it.
	t.Setenv("DISCORD_MANUAL_DEFAULT_PROJECT", "cursemark")
	var wg sync.WaitGroup
	errs := make(chan string, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := manualRequest(r, "POST", "/manual", token, raw)
			if w.Code != 200 {
				errs <- fmt.Sprintf("%d %s", w.Code, w.Body)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	var n int64
	models.AnalyticsDB.Model(&models.Feedback{}).Count(&n)
	if n != 1 {
		t.Fatalf("duplicates: %d", n)
	}
	models.AnalyticsDB.Model(&models.DiscordSyncSource{}).Count(&n)
	if n != 0 {
		t.Fatal("manual flag created a monitored source")
	}
	var s discordSnapshot
	json.Unmarshal([]byte(raw), &s)
	if _, e := sourceForSnapshot(s); e == nil {
		t.Fatal("passive source validation accepted unmonitored channel")
	}
	if _, e := storeDiscordSnapshot(models.DiscordSyncSource{ID: "99100", ProjectID: "necrovale"}, []byte(raw), false); e == nil {
		t.Fatal("passive storage accepted manual source")
	}
	if w := manualRequest(r, "GET", "/messages/"+id+"/status", token, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"durable":true`) {
		t.Fatalf("manual status %d %s", w.Code, w.Body)
	}
	if w := manualRequest(r, "POST", "/messages/"+id+"/mark-result", token, `{"check_marked":true}`); w.Code != 200 {
		t.Fatal(w.Body)
	}
	var item models.DiscordPendingMessage
	models.AnalyticsDB.Take(&item, "id = ?", id)
	if !item.CheckMarked || item.ManualTriggerID != id {
		t.Fatal("manual provenance/check not retained")
	}
	// Retry does not reopen an administratively resolved item.
	models.AnalyticsDB.Model(&fb).Update("resolved", true)
	manualRequest(r, "POST", "/manual", token, raw)
	models.AnalyticsDB.Take(&fb, fb.ID)
	if !fb.Resolved {
		t.Fatal("resolved feedback reopened")
	}
}
func TestDiscordManualKnownDisabledSourceAndMissingDefault(t *testing.T) {
	r, token := manualFixture(t)
	t.Setenv("DISCORD_MANUAL_DEFAULT_PROJECT", "")
	source := models.DiscordSyncSource{ID: "99200", ProjectID: "cursemark", GuildID: models.LegacyDiscordGuild, Kind: "discord_text", Enabled: false, BaselineAt: time.Now().UTC()}
	if e := models.AnalyticsDB.Create(&source).Error; e != nil {
		t.Fatal(e)
	}
	if w := manualRequest(r, "POST", "/manual", token, manualBody("99201", source.ID)); w.Code != 200 {
		t.Fatalf("disabled monitoring blocked explicit flag %d %s", w.Code, w.Body)
	}
	if w := manualRequest(r, "POST", "/manual", token, manualBody("99301", "99300")); w.Code != 409 {
		t.Fatalf("missing default: %d", w.Code)
	}
	t.Setenv("DISCORD_MANUAL_DEFAULT_PROJECT", "does-not-exist")
	if w := manualRequest(r, "POST", "/manual", token, manualBody("99301", "99300")); w.Code != 409 {
		t.Fatalf("unknown project: %d", w.Code)
	}
	var n int64
	models.AnalyticsDB.Model(&models.Project{}).Count(&n)
	if n != 2 {
		t.Fatal("invented project")
	}
	catalog := manualRequest(r, "GET", "/sources", token, "")
	var result struct {
		Items  []models.DiscordSyncSource `json:"items"`
		Manual struct {
			Sources []models.DiscordSyncSource `json:"sources"`
		} `json:"manual"`
	}
	json.Unmarshal(catalog.Body.Bytes(), &result)
	if len(result.Items) != 0 || len(result.Manual.Sources) != 1 {
		t.Fatal("passive/manual source catalogs not separate")
	}
}
func TestDiscordManualAuthGuildIdentityAndForum(t *testing.T) {
	r, token := manualFixture(t)
	t.Setenv("DISCORD_MANUAL_DEFAULT_PROJECT", "cursemark")
	raw := manualBody("99401", "99400")
	if w := manualRequest(r, "POST", "/manual", "wrong", raw); w.Code != 401 {
		t.Fatal("missing auth accepted")
	}
	for _, bad := range []string{strings.Replace(raw, models.LegacyDiscordGuild, "999", 1), strings.Replace(raw, `"kind":"discord_text"`, `"kind":"DM"`, 1)} {
		if w := manualRequest(r, "POST", "/manual", token, bad); w.Code == 200 {
			t.Fatal("foreign guild or DM accepted")
		}
	}
	var data map[string]interface{}
	json.Unmarshal([]byte(raw), &data)
	data["kind"] = "forum_thread"
	data["parent_id"] = "99500"
	data["channel_id"] = "99401"
	data["scope_id"] = "99500"
	data["trigger_message"] = map[string]string{"id": "99402"}
	encoded, _ := json.Marshal(data)
	for i := 0; i < 2; i++ {
		if w := manualRequest(r, "POST", "/manual", token, string(encoded)); w.Code != 200 {
			t.Fatalf("forum %d %s", w.Code, w.Body)
		}
	}
	var n int64
	models.AnalyticsDB.Model(&models.Feedback{}).Count(&n)
	if n != 1 {
		t.Fatalf("forum duplicate %d", n)
	}
	var item models.DiscordPendingMessage
	models.AnalyticsDB.Take(&item, "id = ?", "99401")
	if item.ManualTriggerID != "99402" || item.Kind != "forum_thread" {
		t.Fatal("reply not canonicalized")
	}
}

func TestDiscordManualNewTriggerFencesStaleCheckAcknowledgement(t *testing.T) {
	r, token := manualFixture(t)
	t.Setenv("DISCORD_MANUAL_DEFAULT_PROJECT", "cursemark")
	var body map[string]interface{}
	json.Unmarshal([]byte(manualBody("99701", "99700")), &body)
	body["trigger_message"] = map[string]string{"id": "99702"}
	raw, _ := json.Marshal(body)
	if w := manualRequest(r, "POST", "/manual", token, string(raw)); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w := manualRequest(r, "POST", "/messages/99701/mark-result", token, `{"check_marked":true,"trigger_message_id":"99702"}`); w.Code != 200 {
		t.Fatal(w.Body)
	}
	body["trigger_message"] = map[string]string{"id": "99703"}
	raw, _ = json.Marshal(body)
	if w := manualRequest(r, "POST", "/manual", token, string(raw)); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w := manualRequest(r, "POST", "/messages/99701/mark-result", token, `{"check_marked":true,"trigger_message_id":"99702"}`); w.Code != 409 {
		t.Fatal("stale acknowledgement accepted")
	}
	var current models.DiscordPendingMessage
	models.AnalyticsDB.Take(&current, "id = ?", "99701")
	if current.CheckMarked || current.ManualTriggerID != "99703" {
		t.Fatal("new check retry lost")
	}
	var n int64
	if e := discordQueueScope(models.AnalyticsDB.Model(&models.DiscordPendingMessage{})).Where("check_marked = ?", false).Count(&n).Error; e != nil || n != 1 {
		t.Fatalf("manual retry queue %d %v", n, e)
	}
	for i := 0; i < 2; i++ {
		if w := manualRequest(r, "POST", "/messages/99701/mark-result", token, `{"check_marked":true,"trigger_message_id":"99703"}`); w.Code != 200 {
			t.Fatalf("idempotent mark %d %s", w.Code, w.Body)
		}
	}
	models.AnalyticsDB.Model(&models.Feedback{}).Count(&n)
	if n != 1 {
		t.Fatal("extra feedback")
	}
}

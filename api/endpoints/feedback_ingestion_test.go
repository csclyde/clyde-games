package endpoints

import (
	"api.clyde.games/models"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSourceFailureRetainsCheckpointAndOverlap(t *testing.T) {
	_, m := mcpMockDB(t)
	last := time.Date(2026, 11, 1, 13, 0, 0, 0, time.UTC)
	m.ExpectQuery("SELECT .* FROM `sync_states`").WithArgs("steam_reviews_cursemark").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "last_success_at"}).AddRow(9, "steam_reviews_cursemark", last))
	m.ExpectBegin()
	m.ExpectExec("INSERT INTO `sync_states`").WillReturnResult(sqlmock.NewResult(9, 1))
	m.ExpectCommit()
	m.ExpectBegin()
	m.ExpectExec("UPDATE `sync_states`").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	i, s, from, _, err := runFeedbackSource(models.Project{ID: "cursemark"}, "steam_reviews", "", func(from, to time.Time) (int, int, error) {
		if !from.Equal(last.Add(-time.Hour)) {
			t.Fatalf("overlap: %s", from)
		}
		return 2, 3, errors.New("upstream incomplete")
	})
	if err == nil || i != 2 || s != 3 || !from.Equal(last.Add(-time.Hour)) {
		t.Fatalf("failure contract: %d %d %s %v", i, s, from, err)
	}
}
func TestSteamStoreSuccessAndFailureEnvelopes(t *testing.T) {
	for _, tc := range []struct {
		body string
		ok   bool
	}{
		{`{"success":1,"reviews":[]}`, true}, {`{"success":0}`, false}, {`{"response":{"success":0}}`, false}, {`{}`, false},
	} {
		c := http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})}
		_, err := fetchSteamReviews("https://test.invalid", c)
		if (err == nil) != tc.ok {
			t.Fatalf("%s: %v", tc.body, err)
		}
	}
}
func TestSteamPaginationUsesOneBasedOffsets(t *testing.T) {
	if pagedSteamForumURL("https://steam.test/forum/", 2) != "https://steam.test/forum/?fp=2" || pagedSteamTopicURL("https://steam.test/topic/", 3) != "https://steam.test/topic/?ctp=3" {
		t.Fatal("pagination skipped second page")
	}
}
func TestDiscordInvalidIDsAndDispositionsFailBeforeDatabase(t *testing.T) {
	r, _ := mcpMockDB(t)
	for _, path := range []string{"discord/messages/no", "discord/messages/123/../../4"} {
		w := mcpRequest(r, "GET", path)
		if w.Code != 400 && w.Code != 404 {
			t.Fatalf("invalid route %s: %d", path, w.Code)
		}
	}
}

func TestDiscordBatchReturnsTogetherAndRetainsNoiseIndex(t *testing.T) {
	r, m := mcpMockDB(t)
	m.ExpectQuery("SELECT `id` FROM `discord_pending_messages` WHERE .*scope_id IN").WithArgs(true, models.LegacyDiscordGuild, true, "pending", "imported", false, false).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("123"))
	m.ExpectQuery("SELECT .* FROM `discord_pending_messages` WHERE id IN").WithArgs("123").WillReturnRows(sqlmock.NewRows([]string{"id", "snapshot", "disposition"}).AddRow("123", `{"id":"123","content":"bug evidence"}`, "pending"))
	m.ExpectQuery("SELECT `id` FROM `discord_pending_messages` WHERE .*scope_id IN").WithArgs(true, models.LegacyDiscordGuild, "noise").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("124"))
	m.ExpectQuery("SELECT .* FROM `discord_sync_sources`").WithArgs(true).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	w := mcpRequest(r, "GET", "discord/batch")
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var response struct {
		Snapshots []json.RawMessage `json:"snapshots"`
		NoiseIDs  []string          `json:"noise_ids"`
		Complete  bool              `json:"snapshot_summaries_complete"`
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil || len(response.Snapshots) != 1 || len(response.NoiseIDs) != 1 || !response.Complete {
		t.Fatalf("batch: %s", w.Body)
	}
}

func TestFeedbackTriagePreservesMessageAndResolvedState(t *testing.T) {
	r, m := mcpMockDB(t)
	m.ExpectQuery("SELECT .* FROM `feedbacks`").WithArgs(12).WillReturnRows(sqlmock.NewRows([]string{"id", "message", "resolved"}).AddRow(12, "original", true))
	m.ExpectBegin()
	m.ExpectExec("UPDATE `feedbacks` SET `translated`=\\?,`triage_at`=\\?,`triage_note`=\\?,`triage_state`=\\?,`updated_at`=\\? WHERE").WithArgs("translation", sqlmock.AnyArg(), "reason", "noise", sqlmock.AnyArg(), 12).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	req := httptest.NewRequest("POST", "/internal/mcp/feedback/12/triage", strings.NewReader(`{"state":"noise","note":"reason","translated":"translation"}`))
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"resolved":true`) {
		t.Fatalf("triage: %d %s", w.Code, w.Body)
	}
}

var _ = gin.H{}

func TestSteamReviewFallbackUsesExistingWebAPIWhenStoreRedirects(t *testing.T) {
	calls := 0
	client := http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		body := "<html>Steam homepage</html>"
		if r.URL.Host == "api.steampowered.com" {
			var input map[string]interface{}
			if err := json.Unmarshal([]byte(r.URL.Query().Get("input_json")), &input); err != nil {
				t.Fatal(err)
			}
			if input["filter"] != float64(1) || input["purchase_type"] != float64(1) || input["cursor"] != "cursor+1" {
				t.Fatalf("fallback inputs: %+v", input)
			}
			body = `{"response":{"reviews":[],"cursor":"next"}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	page, err := fetchSteamReviewsPage(client, "3219180", "cursor+1", 2)
	if err != nil || page.Cursor != "next" || calls != 2 {
		t.Fatalf("fallback: %+v %v calls=%d", page, err, calls)
	}
}

func TestDiscordDispositionReplayIsIdempotent(t *testing.T) {
	r, m := mcpMockDB(t)
	m.ExpectQuery("SELECT .* FROM `discord_pending_messages` WHERE id").WithArgs("123").WillReturnRows(sqlmock.NewRows([]string{"id", "scope_id", "project"}).AddRow("123", "456", "game"))
	m.ExpectQuery("SELECT .* FROM `discord_sync_sources`").WithArgs("456", "game", true).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("456"))
	m.ExpectBegin()
	m.ExpectQuery("SELECT .* FROM `discord_pending_messages`.*FOR UPDATE").WithArgs("123").WillReturnRows(sqlmock.NewRows([]string{"id", "disposition", "note"}).AddRow("123", "noise", "greeting"))
	m.ExpectCommit()
	req := httptest.NewRequest("POST", "/internal/mcp/discord/messages/123/disposition", strings.NewReader(`{"disposition":"noise","note":"greeting"}`))
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 32))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("replay %d %s", w.Code, w.Body)
	}
}
func TestBotTokenCannotAuthorizeAdmin(t *testing.T) {
	r, _ := mcpMockDB(t)
	t.Setenv("DISCORD_INGEST_TOKEN", strings.Repeat("b", 32))
	req := httptest.NewRequest("GET", "/internal/mcp/feedback", nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("b", 32))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatal("bot credential authorized admin route")
	}
}

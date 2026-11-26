package endpoints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

type marketingTransport func(*http.Request) (*http.Response, error)

func (f marketingTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMarketingHTTPAndSteamworks(t *testing.T) {
	db := syncTestDB(t)
	if err := models.MigrateMarketing(db); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.Project{ID: "cursemark", Name: "Cursemark", SteamAppID: "3219180"}).Error; err != nil {
		t.Fatal(err)
	}
	t.Setenv("PORTAL_AUTH_MODE", "legacy")
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("p", 36))
	t.Setenv("CURSEMARK_OPENAI_API_KEY", "")
	router := gin.New()
	RegisterMarketingRoutes(router)
	const base = "/marketing/cursemark"
	call := func(method, path string, payload any, authorized bool, status int, out any) {
		t.Helper()
		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if authorized {
			req.Header.Set("X-Admin-API-Token", strings.Repeat("p", 36))
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s %s: got %d expected %d: %s", method, path, res.Code, status, res.Body)
		}
		if out != nil {
			if err := json.Unmarshal(res.Body.Bytes(), out); err != nil {
				t.Fatal(err)
			}
		}
	}
	call("GET", base+"/beats", nil, false, 401, nil)
	call("GET", "/marketing/necrovale/beats", nil, true, 404, nil)
	call("POST", "/marketing/farm-builder/reports", nil, true, 404, nil)
	const beatsPath = base + "/beats"
	call("POST", beatsPath, map[string]any{"name": "Missing CTA", "brief": "Example"}, true, 400, nil)
	call("POST", beatsPath, map[string]any{"name": "Wrong CTA", "brief": "Example", "call_to_action": "demo"}, true, 400, nil)
	var fresh marketingBeatView
	call("POST", beatsPath, marketingBeatDraft{Name: "New Beat", Brief: "A changed spell", CallToAction: "wishlist"}, true, 201, &fresh)
	if fresh.ID == 0 || fresh.CallToAction != "wishlist" || fresh.Version != 1 {
		t.Fatalf("Three-field creation did not persist: %+v", fresh)
	}
	legacy := models.MarketingBeat{Project: marketingProject, Version: 1, MarketingBrief: models.MarketingBrief{
		Name: "Storm runes", AssetKind: "video", Asset: "https://assets.example/private-token",
		Hook: "Initial hook", WhyNow: "A legacy occasion", CTA: "Wishlist Cursemark on Steam", Goal: "wishlists", GoalTarget: 10,
	}}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	var list struct {
		Beats []marketingBeatView `json:"beats"`
	}
	call("GET", beatsPath, nil, true, 200, &list)
	if len(list.Beats) != 2 || list.Beats[0].ID != legacy.ID || list.Beats[0].Brief != legacy.Hook ||
		list.Beats[0].CallToAction != "wishlist" {
		t.Fatalf("Legacy hook and CTA fallback are unavailable: %+v", list)
	}
	beat := list.Beats[0]
	path := fmt.Sprintf("%s/beats/%d", base, beat.ID)
	call("POST", path+"/generate", map[string]any{"version": beat.Version}, true, 503, nil)
	beat.Brief = "Updated hook"
	beat.CallToAction = "purchase"
	call("PUT", path, beat, true, 200, &beat)
	if beat.Brief != "Updated hook" || beat.Version != 2 || beat.CallToAction != "purchase" {
		t.Fatalf("Beat update did not persist: %+v", beat)
	}
	call("PUT", path, map[string]any{"name": beat.Name, "brief": "old", "call_to_action": "wishlist", "version": 1}, true, 409, nil)
	stored, err := readMarketingBeat(db, legacy.ID)
	expectedBrief := legacy.MarketingBrief
	expectedBrief.Brief = beat.Brief
	if err != nil || stored.MarketingBrief != expectedBrief {
		t.Fatalf("Editing the new Beat fields changed legacy metadata: %+v %v", stored.MarketingBrief, err)
	}

	t.Setenv("CURSEMARK_OPENAI_API_KEY", "mock-secret")
	oldClient := marketingAIClient
	t.Cleanup(func() { marketingAIClient = oldClient })
	marketingAIClient = &http.Client{Transport: marketingTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.openai.com/v1/responses" || r.Header.Get("Authorization") != "Bearer mock-secret" {
			t.Fatal("Unexpected OpenAI request")
		}
		raw, _ := io.ReadAll(r.Body)
		var request struct {
			Input string `json:"input"`
		}
		var input map[string]string
		if json.Unmarshal(raw, &request) != nil || json.Unmarshal([]byte(request.Input), &input) != nil ||
			len(input) != 3 || input["brief"] != "Updated hook" || input["name"] != beat.Name || input["call_to_action"] != "purchase" ||
			!bytes.Contains(raw, []byte("untrusted evidence")) || !bytes.Contains(raw, []byte(`"store":false`)) {
			t.Fatalf("Unexpected research context: %s", raw)
		}
		primary := "https://example.org/community/rules"
		first := marketingOpportunity{Platform: "Forum", Community: "Roguelites", AudienceFit: "Video fits", Timing: "Friday 18:00 UTC",
			Title: "A rune build", Copy: "Clip {steam_link}", AssetNotes: "Native video", RulesURL: primary, RuleEvidence: "Disclosure is required", Clear: true}
		second := first
		second.Community, second.RulesURL = "Other forum", "https://example.net/invented-rules"
		answer, _ := json.Marshal(map[string]any{"opportunities": []marketingOpportunity{first, second}})
		response, _ := json.Marshal(map[string]any{"status": "completed", "output": []any{
			map[string]any{"type": "web_search_call", "status": "completed", "action": map[string]any{"sources": []any{map[string]string{"url": primary}}}},
			map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": string(answer)}}},
		}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(response)), Header: make(http.Header)}, nil
	})}
	call("POST", path+"/generate", map[string]any{"version": beat.Version}, true, 201, nil)
	var detail struct {
		Beat       marketingBeatView `json:"beat"`
		Placements []marketingPost   `json:"placements"`
		Result     marketingSummary  `json:"result"`
	}
	call("GET", path, nil, true, 200, &detail)
	if len(detail.Placements) != 2 || detail.Placements[0].Decision != "candidate" ||
		!detail.Placements[0].SourceCited || detail.Placements[0].Rank != 1 ||
		detail.Placements[1].Decision != "hold" || detail.Placements[1].RulesURL != "" {
		t.Fatalf("Missing ranked drafts with source gating: %+v", detail.Placements)
	}
	post, held := detail.Placements[0], detail.Placements[1]
	link, _ := url.Parse(post.SteamURL)
	content := link.Query().Get("utm_content")
	if !regexp.MustCompile("^[0-9a-f]{32}$").MatchString(content) || post.SteamURL == held.SteamURL {
		t.Fatal("Placements must have unique Steam links")
	}
	at := time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)
	post.PublishedAt, post.PublishedURL = &at, "https://example.org/post/123"
	post.PublishedNote, post.Copy, post.RuleEvidence = "Posted manually", "Edited in portal", "Checked by admin"
	call("PUT", fmt.Sprintf("%s/placements/%d", path, post.ID), post, true, 200, &post)
	if post.Copy != "Edited in portal" || post.SourceCited || post.PublishedAt == nil || post.Version != 2 || post.SteamURL != link.String() {
		t.Fatalf("Draft edit/publication or tracking identity mismatch: %+v", post)
	}
	const headers = "\ufeffApp ID,UTM Source,UTM Medium,UTM Campaign,UTM Content,Total Visits,Trusted Visits,Tracked Visits,Wishlists,Purchases,Activations\n"
	csv := func(wishlists int) string {
		return headers + fmt.Sprintf("3219180,clydegames,community,%s,%s,100,55,25,%d,0,\n", marketingCampaign(beat.ID), content, wishlists)
	}
	importFile := func(start, end, data string, status int, duplicate bool) {
		t.Helper()
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		_ = w.WriteField("start_date", start)
		_ = w.WriteField("end_date", end)
		_ = w.WriteField("full_breakdown", "yes")
		file, _ := w.CreateFormFile("csv", "utm.csv")
		_, _ = io.WriteString(file, data)
		_ = w.Close()
		req := httptest.NewRequest("POST", base+"/reports", &body)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("X-Admin-API-Token", strings.Repeat("p", 36))
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != status || (status == 200 && !strings.Contains(res.Body.String(), fmt.Sprintf(`"duplicate":%t`, duplicate))) {
			t.Fatalf("Import %s/%s: %d %s", start, end, res.Code, res.Body)
		}
	}
	importFile("2025-01-01", "2025-01-07", csv(2), 200, false)
	importFile("2025-01-01", "2025-01-07", csv(2), 200, true)
	importFile("2025-01-04", "2025-01-10", csv(99), 409, false)
	importFile("2025-01-01", "2025-01-07", csv(3), 200, false)
	importFile("2025-01-01", "2025-01-07", csv(2), 200, true)
	importFile("2025-01-08", "2025-01-12", csv(4), 200, false)
	importFile("2025-02-01", "2025-02-10", strings.ReplaceAll(csv(8), "3219180", "1234567"), 400, false)
	detail.Result = marketingSummary{}
	call("GET", path, nil, true, 200, &detail)
	result := detail.Result
	if result.Wishlists == nil || *result.Wishlists != 7 || result.Purchases == nil || *result.Purchases != 0 ||
		result.Activations != nil || result.Tracked == nil || *result.Tracked != 50 || result.Observed != 2 {
		t.Fatalf("Unexpected reconciled results: %+v", result)
	}
	importFile("2025-01-08", "2025-01-12", headers, 200, false)
	detail.Result = marketingSummary{}
	call("GET", path, nil, true, 200, &detail)
	if result = detail.Result; result.Observed != 1 || result.Expected != 2 || result.Wishlists == nil || *result.Wishlists != 3 {
		t.Fatalf("Suppressed row should be missing: %+v", result)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, start := range []string{"2025-03-01", "2025-03-02"} {
		wg.Add(1)
		go func(start string) {
			defer wg.Done()
			_, _, err := saveMarketingReport(db, start, "2025-03-05", strings.Repeat("1", 64), nil)
			errs <- err
		}(start)
	}
	wg.Wait()
	a, b := <-errs, <-errs
	if !((a == nil && b == errMarketingOverlap) || (a == errMarketingOverlap && b == nil)) {
		t.Fatalf("Concurrent overlapping import was not serialized: %v, %v", a, b)
	}
}

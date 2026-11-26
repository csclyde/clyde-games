package endpoints

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func TestMarketingImpactBands(t *testing.T) {
	for _, test := range []struct {
		metric string
		value  int64
		level  int
	}{
		{"youtube_views", 0, 1}, {"youtube_views", 999, 1}, {"youtube_views", 1_000, 2},
		{"youtube_views", 9_999, 2}, {"youtube_views", 10_000, 3},
		{"youtube_short_views", 2_999, 1}, {"youtube_short_views", 3_000, 2},
		{"youtube_short_views", 29_999, 2}, {"youtube_short_views", 30_000, 3},
		{"reddit_score", -1, 1}, {"reddit_score", 9, 1}, {"reddit_score", 10, 2},
		{"reddit_score", 49, 2}, {"reddit_score", 50, 3},
	} {
		item := marketingImpactObservation{Metric: test.metric, Scope: "dedicated", Value: &test.value}
		if level := impactBand(item); level != test.level {
			t.Fatalf("%s %d: got %d, want %d", test.metric, test.value, level, test.level)
		}
		item.Scope = "shared"
		if impactBand(item) != 0 {
			t.Fatal("shared counts influenced score")
		}
		item.Scope, item.Value = "dedicated", nil
		if impactBand(item) != 0 {
			t.Fatal("missing count interpreted as zero")
		}
	}
	for raw, want := range map[string]string{
		"https://old.reddit.com/r/ARPG/":          "reddit",
		"http://REDDIT.com/r/ARPG":                "reddit",
		"https://www.reddit.com.evil.test/r/ARPG": "",
		"https://evil.test/reddit.com":            "",
		"https://notreddit.com/r/ARPG/":           "",
		"https://www.reddit.com@evil.test/r/ARPG": "",
		"https://evil.test@reddit.com/r/ARPG":     "",
		"https://twitter.com/sample":              "x",
		"https://www.youtube.com/@sample":         "youtube",
	} {
		if p := marketingPlatform(raw); p != want {
			t.Fatalf("%s classified as %q, wanted %q", raw, p, want)
		}
	}
}

func TestMarketingTargetImpactRoutes(t *testing.T) {
	db := syncTestDB(t)
	if err := models.MigrateMarketing(db); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PORTAL_AUTH_MODE", "legacy")
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("a", 36))
	router := gin.New()
	RegisterMarketingRoutes(router)
	const path = "/marketing/cursemark/targets"
	target := models.MarketingTarget{Project: marketingProject, Version: 1, Name: "Legacy Reddit",
		URL: "http://old.reddit.com/r/Roguelites", URLKey: "legacy-test"}
	if err := db.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	one := path + "/" + strconv.FormatUint(uint64(target.ID), 10)
	route := func(method, endpoint, body string, want int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, endpoint, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Admin-API-Token", strings.Repeat("a", 36))
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("%s %s returned %d want %d: %s", method, endpoint, res.Code, want, res.Body)
		}
		return res.Body.Bytes()
	}
	get := func() marketingTargetView {
		t.Helper()
		var detail marketingTargetView
		if err := json.Unmarshal(route("GET", one, "", 200), &detail); err != nil {
			t.Fatal(err)
		}
		return detail
	}
	detail := get()
	if detail.Platform != "reddit" || detail.Impact.Level != 0 || detail.ImpactEvidence.Version != 0 || detail.SteamAttribution != nil {
		t.Fatalf("bad initial detail: %+v", detail)
	}
	first := `{"metric":"reddit_score","url":"https://www.reddit.com/r/roguelites/comments/1msh1gr","title":"Cursemark post","observed_on":"2025-08-17","value":0,"scope":"dedicated"}`
	alias := strings.Replace(first, "www.reddit.com/r/roguelites/comments/1msh1gr", "old.reddit.com/r/ROGUELITES/comments/1msh1gr/slug", 1)
	route("PUT", one+"/impact", `{"version":0,"observations":[`+first+","+alias+"]}", 400)
	wrong := strings.Replace(first, "/roguelites/", "/games/", 1)
	route("PUT", one+"/impact", `{"version":0,"observations":[`+wrong+"]}", 400)
	route("PUT", strings.Replace(one, "cursemark", "necrovale", 1)+"/impact", `{"version":0,"observations":[]}`, 404)
	zero := `{"version":0,"observations":[` + first + "]}"
	route("PUT", one+"/impact", zero, 200)
	route("PUT", one+"/impact", zero, 409)
	if detail = get(); detail.Impact.Level != 1 || detail.ImpactEvidence.Items[0].Value == nil || *detail.ImpactEvidence.Items[0].Value != 0 {
		t.Fatalf("verified zero lost: %+v", detail.Impact)
	}
	raw := route("GET", path, "", 200)
	if strings.Contains(string(raw), "impact_evidence") || !strings.Contains(string(raw), `"level":1`) {
		t.Fatalf("catalog should expose only impact summary: %s", raw)
	}
	unknown := strings.Replace(first, `"value":0`, `"value":null`, 1)
	route("PUT", one+"/impact", `{"version":1,"observations":[`+unknown+"]}", 200)
	if detail = get(); detail.Impact.Level != 0 || detail.ImpactEvidence.Items[0].Value != nil || detail.ImpactEvidence.Version != 2 {
		t.Fatalf("unknown value lost: %+v", detail.Impact)
	}
	if detail.NewStatus != nil {
		t.Fatal("score updates must not mark a legacy target new")
	}
	route("PUT", one+"/history", `{"version":0,"history":{"outreach_status":"none_found","activity_status":"none_found","checked_on":"2025-08-17"}}`, 400)
	route("PUT", one+"/history", `{"version":0,"history":{"outreach_status":"not_applicable","activity_status":"none_found","checked_on":"2025-08-17"}}`, 200)
	if detail = get(); detail.NewStatus == nil || !detail.NewStatus.IsNew || detail.History.OutreachStatus != "not_applicable" || detail.Impact.Level != 0 {
		t.Fatalf("checked absence should set New independently of impact: %+v", detail)
	}
	creatorBody := `{"name":"Public-only creator","kind":"creator","platform":"youtube","channel_urls":["https://www.youtube.com/@ImpactFixture"]}`
	creatorRaw := route("POST", path, creatorBody, 201)
	var creator marketingTargetView
	if err := json.Unmarshal(creatorRaw, &creator); err != nil || creator.Creator == nil || creator.Creator.Email != "" {
		t.Fatal("public creator must be saved without inventing mail")
	}
	duplicate := strings.Replace(creatorBody, "www.youtube.com/@ImpactFixture", "youtube.com/@impactfixture/", 1)
	route("POST", path, duplicate, 409)
	creatorPath := path + "/" + strconv.FormatUint(uint64(creator.ID), 10)
	route("PUT", creatorPath+"/history", `{"version":0,"history":{"outreach_status":"identity_unverified","activity_status":"not_checked","checked_on":"2025-08-17"}}`, 200)
	unknownCreator := route("GET", creatorPath, "", 200)
	if strings.Contains(string(unknownCreator), `"new_status"`) {
		t.Fatal("unverified address with unreviewed coverage must not imply New")
	}
	route("PUT", creatorPath+"/creator-evidence", `{"version":0,"coverage":{"status":"no_result","checked_at":"2025-08-17","items":[]}}`, 200)
	if !strings.Contains(string(route("GET", creatorPath, "", 200)), `"label":"No prior coverage found"`) {
		t.Fatal("searched coverage must be independent of unverified outreach")
	}
}

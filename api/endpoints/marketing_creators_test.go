package endpoints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"

	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api.clyde.games/models"
	"github.com/gin-gonic/gin"
)

func TestMarketingCreatorImportAndPrivacy(t *testing.T) {
	db := syncTestDB(t)
	if err := models.MigrateMarketing(db); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PORTAL_AUTH_MODE", "legacy")
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("a", 36))

	router := gin.New()
	RegisterMarketingRoutes(router)
	const path = "/marketing/cursemark/targets"
	route := func(method, endpoint string, body io.Reader, content, token string, status int) []byte {
		t.Helper()
		req := httptest.NewRequest(method, endpoint, body)
		if content != "" {
			req.Header.Set("Content-Type", content)
		}
		if token != "" {
			req.Header.Set("X-Admin-API-Token", token)
		}
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != status {
			t.Fatalf("%s %s returned %d, want %d", method, endpoint, res.Code, status)
		}
		return res.Body.Bytes()
	}
	admin := strings.Repeat("a", 36)
	source := "name,email,platform,channel_urls,games_covered\n" +
		"Private display name,mock-contact@example.invalid,youtube,https://youtube.com/@fixture?email=secret@example.invalid,Manually noted game\n"
	upload := func(token, data string, status int) []byte {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		file, _ := writer.CreateFormFile("csv", "creators.csv")
		_, _ = io.WriteString(file, data)
		_ = writer.Close()
		return route("POST", path+"/import-creators", &body, writer.FormDataContentType(), token, status)
	}
	upload("", source, 401)
	receipt := upload(admin, source, 200)
	if !bytes.Contains(receipt, []byte(`"added":1`)) || bytes.Contains(receipt, []byte("@")) {
		t.Fatal("Bad import receipt")
	}
	if raw := upload(admin, strings.ReplaceAll(source, "mock-contact", "MOCK-CONTACT"), 200); !bytes.Contains(raw, []byte(`"duplicates":1`)) {
		t.Fatal("Case-only duplicate was imported")
	}
	upload(admin, source+"Bad row,invalid,twitch,,\n", 400)
	raw := route("GET", path, nil, "", admin, 200)
	if bytes.Contains(raw, []byte("mock-contact")) || bytes.Contains(raw, []byte("Manually noted game")) || bytes.Contains(raw, []byte("secret@")) {
		t.Fatal("Contact fields appeared in the catalog response")
	}
	var list struct{ Targets []marketingTargetView }
	if json.Unmarshal(raw, &list) != nil || len(list.Targets) != 1 || list.Targets[0].Kind != "creator" {
		t.Fatal("Unexpected creator catalog")
	}
	target := list.Targets[0]
	one := fmt.Sprintf("%s/%d", path, target.ID)
	route("GET", one, nil, "", "", 401)
	route("GET", strings.Replace(one, "cursemark", "necrovale", 1), nil, "", admin, 404)
	raw = route("GET", one, nil, "", admin, 200)
	var detail marketingTargetView
	if json.Unmarshal(raw, &detail) != nil || detail.Creator == nil || detail.Creator.Email != "mock-contact@example.invalid" ||
		len(detail.Creator.ChannelURLs) != 1 || detail.Creator.ChannelURLs[0] != "https://www.youtube.com/@fixture" {
		t.Fatal("Private detail or channel canonicalization was incorrect")
	}
	t.Setenv("MCP_UPSTREAM_TOKEN", strings.Repeat("m", 36))
	RegisterMCPRoutes(router)
	// The signed-in portal reads the private contact; the MCP role gets the
	// record and evidence with the address removed.
	coverage := `{"version":0,"coverage":{"status":"covered","checked_at":"2025-01-02","items":[{"title":"Cursemark gameplay","url":"https://www.youtube.com/watch?v=example","date":"2025-01-01","format":"video"}]},"reply":{"status":"no_reply_found","checked_at":"2025-01-02","note":"No verified reply found."}}`
	raw = route("PUT", one+"/creator-evidence", strings.NewReader(coverage), "application/json", admin, 200)
	if bytes.Contains(raw, []byte("mock-contact")) || !bytes.Contains(raw, []byte("Cursemark gameplay")) {
		t.Fatal("Private evidence response undisclosed or invalid")
	}
	route("PUT", one+"/creator-evidence", strings.NewReader(coverage), "application/json", admin, 409)
	route("PUT", one+"/creator-evidence", strings.NewReader(`{"version":1,"reply":{"status":"no_reply_found","checked_at":"2025-01-02","note":"private@example.invalid"}}`), "application/json", admin, 400)
	official := "https://www.youtube.com/@fixture"
	data, _ := json.Marshal(map[string]any{"version": 1, "research": marketingResearch{Sources: []string{official},
		Examples: []marketingExample{{Title: "Public coverage", URL: "https://www.youtube.com/watch?v=public", Date: time.Now().UTC().Format(marketingDay), WhatWorked: "private-leak@example.invalid"}},
		Games:    []string{"Into the Necrovale", "fake-contact@example.invalid"}}})
	raw = route("PUT", one+"/research", bytes.NewReader(data), "application/json", admin, 200)
	if json.Unmarshal(raw, &detail) != nil || !detail.ResearchReady || len(detail.Research.Examples) != 1 ||
		len(detail.Research.Games) != 1 || detail.Evidence == nil || detail.Evidence.Version != 1 ||
		bytes.Contains(raw, []byte("private-leak")) || bytes.Contains(raw, []byte("fake-contact")) {
		t.Fatal("Manual research replaced separate evidence or leaked private fields")
	}
	mcpPath := "/internal/mcp" + one
	req := httptest.NewRequest("GET", mcpPath, nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("m", 36))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	if response.Code != 200 || !strings.Contains(response.Body.String(), "creator_evidence") || strings.Contains(response.Body.String(), "mock-contact") {
		t.Fatal("MCP creator detail improperly scoped")
	}
	var stored models.MarketingCreator
	if db.First(&stored, target.ID).Error != nil || stored.Email != "mock-contact@example.invalid" {
		t.Fatal("Research altered the private address")
	}

}

func TestMarketingPrivateDraftPolicy(t *testing.T) {
	const target = "https://www.reddit.com/r/fixture/"
	const official = target + "about/rules"
	research := marketingResearch{Sources: []string{official}, AIRestriction: "unspecified",
		Rules: marketingRules{Links: "Allowed", SelfPromotion: "Allowed", AI: "Unclear", Flair: "Required", Cadence: "Monthly"}}
	run := func(want bool) {
		data, _ := json.Marshal(research)
		result, err := validateResearch(target, data, map[string]bool{marketingSourceKey(official): true}, time.Now().UTC())
		if err != nil || result.Hold != want {
			t.Fatalf("Hold = %v, want %v", result.Hold, want)
		}
	}
	run(false)
	research.AIRestriction = "final_content"
	research.Rules.AI = "Final submissions must be human-written."
	run(false)
	research.AIRestriction = "assisted_submissions"
	research.Rules.AI = "No AI may be used to prepare a submission."
	run(true)
	research.AIRestriction = "unspecified"
	research.Rules.SelfPromotion = "Unclear"
	run(true)
}

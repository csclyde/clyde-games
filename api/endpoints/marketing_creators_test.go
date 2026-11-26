package endpoints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
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
	t.Setenv("CURSEMARK_OPENAI_API_KEY", "test-key")
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
	old := marketingAIClient
	t.Cleanup(func() { marketingAIClient = old })
	calls := 0
	marketingAIClient = &http.Client{Transport: marketingTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		var body struct{ Input string }
		if json.NewDecoder(req.Body).Decode(&body) != nil {
			t.Fatal("Bad provider request")
		}
		for _, secret := range []string{"mock-contact", "secret@", "Private display name", "Manually noted game", "correspondence"} {
			if strings.Contains(body.Input, secret) {
				t.Fatal("Private data reached the OpenAI request")
			}
		}
		if !strings.Contains(body.Input, "https://www.youtube.com/@fixture") {
			t.Fatal("Missing public channel")
		}
		official := "https://www.youtube.com/@fixture"
		post := "https://www.youtube.com/watch?v=public"
		answer, _ := json.Marshal(marketingResearch{Sources: []string{official},
			Examples: []marketingExample{{Title: "Public coverage", URL: post, Date: time.Now().UTC().Format(marketingDay), WhatWorked: "private-leak@example.invalid"}},
			Games:    []string{"Into the Necrovale", "fake-contact@example.invalid"}})
		output, _ := json.Marshal(map[string]any{"status": "completed", "output": []any{
			map[string]any{"type": "web_search_call", "status": "completed", "action": map[string]any{"sources": []any{map[string]string{"url": official}, map[string]string{"url": post}}}},
			map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": string(answer)}}},
		}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(output)), Header: make(http.Header)}, nil
	})}
	if calls != 0 {
		t.Fatal("Import unexpectedly triggered paid research")
	}
	raw = route("POST", one+"/research", strings.NewReader(`{"version":1}`), "application/json", admin, 200)
	if calls != 1 || json.Unmarshal(raw, &detail) != nil || !detail.ResearchReady || len(detail.Research.Examples) != 1 ||
		len(detail.Research.Games) != 1 || bytes.Contains(raw, []byte("private-leak")) || bytes.Contains(raw, []byte("fake-contact")) {
		t.Fatal("Creator research did not sanitize and persist the public evidence")
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

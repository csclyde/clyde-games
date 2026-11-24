package endpoints

import (
	"api.clyde.games/models"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"strings"
	"testing"
)

func TestCrashWorkHTTPClaimsAndLegacyExactRetry(t *testing.T) {
	db := workDB(t)
	a := workReport(t, db, "cursemark", "one", "fault", false)
	b := workReport(t, db, "cursemark", "one", "fault", false)
	t.Setenv("MCP_UPSTREAM_TOKEN", strings.Repeat("a", 32))
	t.Setenv("PORTAL_AUTH_MODE", "legacy")
	t.Setenv("ADMIN_API_TOKEN", strings.Repeat("x", 32))
	r := gin.New()
	RegisterMCPRoutes(r)
	RegisterReportNoteRoutes(r)
	path := func(id uint, suffix string) string {
		return fmt.Sprintf("crash-investigation/%d%s?project=cursemark", id, suffix)
	}
	w := noteRequest(r, "POST", path(a.ID, "/claim"), `{"request_key":"http-run"}`)
	var claim models.CrashClaimResult
	json.Unmarshal(w.Body.Bytes(), &claim)
	if w.Code != 200 || !claim.Acquired || claim.Token == "" {
		t.Fatalf("claim %d", w.Code)
	}
	w = noteRequest(r, "POST", path(b.ID, "/claim"), `{"request_key":"another-run"}`)
	var denied models.CrashClaimResult
	json.Unmarshal(w.Body.Bytes(), &denied)
	if w.Code != 200 || denied.Acquired || denied.Reason != "already_claimed" {
		t.Fatal("alias duplicate claim")
	}
	w = noteRequest(r, "POST", path(a.ID, "/notes"), `{"request_key":"no-claim","note":"Unclaimed work"}`)
	if w.Code != 409 {
		t.Fatal("unclaimed append", w.Code)
	}
	w = noteRequest(r, "POST", path(a.ID, "/notes"), `{"request_key":"actor-forge","note":"x","actor":"admin"}`)
	if w.Code != 400 {
		t.Fatal("actor injection", w.Code)
	}
	body, _ := json.Marshal(models.AppendCrashWorkInput{RequestKey: "http-outcome", Note: "Tested local fix; unreleased, owner review pending.", State: "ready_for_review", ReleaseState: "local", ClaimToken: claim.Token, Complete: true})
	w = noteRequest(r, "POST", path(a.ID, "/notes"), string(body))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"resolution_changed":false`) {
		t.Fatal("group outcome", w.Code, w.Body)
	}
	w = noteRequest(r, "POST", path(b.ID, "/notes"), string(body))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"replayed":true`) {
		t.Fatal("alias retry", w.Code)
	}
	w = noteRequest(r, "POST", fmt.Sprintf("report-notes/crash/%d?project=cursemark", a.ID), `{"request_key":"per-report-new","note":"Separate investigation"}`)
	if w.Code != 409 || !strings.Contains(w.Body.String(), `"crash_group_required"`) {
		t.Fatal("legacy initiation allowed", w.Code)
	}
	// An uncertain pre-cutover write can still replay exactly without duplication.
	in := models.AppendReportNoteInput{RequestKey: "legacy-retry", Note: "Original saved context.", State: "blocked"}
	raw, _ := json.Marshal(struct {
		Actor string
		Input models.AppendReportNoteInput
	}{"mcp", in})
	n := models.ReportNote{Kind: "crash", ReportID: a.ID, Project: "cursemark", Actor: "mcp", Note: in.Note, State: in.State, RequestKey: in.RequestKey, PayloadHash: fmt.Sprintf("%x", sha256.Sum256(raw))}
	if e := db.Create(&n).Error; e != nil {
		t.Fatal(e)
	}
	oldBody, _ := json.Marshal(in)
	w = noteRequest(r, "POST", fmt.Sprintf("report-notes/crash/%d?project=cursemark", a.ID), string(oldBody))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"replayed":true`) {
		t.Fatal("lost legacy exact retry", w.Code)
	}
	v, e := models.GetCrashWork(db, a.ID, "cursemark")
	if e != nil || v.State != "ready_for_review" {
		t.Fatal("late legacy history replaced canonical group outcome", e)
	}
	var stored models.Crash
	db.First(&stored, a.ID)
	if stored.Resolved {
		t.Fatal("investigation resolved source")
	}
}

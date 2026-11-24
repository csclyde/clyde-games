package endpoints

import (
	"api.clyde.games/models"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"strings"
	"sync"
	"testing"
	"time"
)

func workDB(t *testing.T) *gorm.DB {
	db := syncTestDB(t)
	if e := models.MigrateReportNotes(db); e != nil {
		t.Fatal(e)
	}
	return db
}
func workReport(t *testing.T, db *gorm.DB, project, key, message string, resolved bool) models.Crash {
	t.Helper()
	now := time.Now().UTC()
	r := models.Crash{Project: project, GroupKey: key, Message: message, Stack: "original evidence", Resolved: resolved, Count: 1, OccurrenceVersion: 1, IngestedAt: &now}
	if e := db.Create(&r).Error; e != nil {
		t.Fatal(e)
	}
	return r
}
func workAppend(t *testing.T, db *gorm.DB, r models.Crash, token, key, state string) models.ReportNote {
	t.Helper()
	n, _, e := models.AppendCrashWork(db, r.ID, r.Project, "mcp", models.AppendCrashWorkInput{RequestKey: key, Note: "Tried locally; unreleased. Owner closes manually.", State: state, ReleaseState: "local", ClaimToken: token, Complete: true})
	if e != nil {
		t.Fatal(e)
	}
	return n
}
func TestCrashWorkLegacyScopeMigrationAndQueue(t *testing.T) {
	db := workDB(t)
	a := workReport(t, db, "cursemark", "real", "fault", false)
	b := workReport(t, db, "Cursemark", "real", "fault", true)
	other := workReport(t, db, "necrovale", "real", "fault", false)
	av1 := workReport(t, db, "cursemark", "av-a", "Access violation A", false)
	av2 := workReport(t, db, "cursemark", "av-b", "Access violation B", false)
	l1 := workReport(t, db, "cursemark", "", "legacy", false)
	l2 := workReport(t, db, "cursemark", "", "legacy", false)
	for _, r := range []models.Crash{b, a} {
		if e := db.Create(&models.ReportNote{Kind: "crash", ReportID: r.ID, Project: "cursemark", Actor: "mcp", RequestKey: "legacy-note", Note: "Original concise findings", State: "blocked"}).Error; e != nil {
			t.Fatal(e)
		}
	}
	snapshot := func() string {
		var c []models.Crash
		var n []models.ReportNote
		db.Unscoped().Order("id").Find(&c)
		db.Order("id").Find(&n)
		raw, _ := json.Marshal([]any{c, n})
		return string(raw)
	}
	before := snapshot()
	for i := 0; i < 2; i++ {
		if e := models.MigrateCrashInvestigations(db); e != nil {
			t.Fatal(e)
		}
	}
	va, e := models.GetCrashWork(db, a.ID, "cursemark")
	if e != nil {
		t.Fatal(e)
	}
	vb, e := models.GetCrashWork(db, b.ID, "cursemark")
	if e != nil || va.Key != vb.Key || va.State != "blocked" || va.NeedsReview || va.NoteCount != 2 || va.ReportCount != 2 {
		t.Fatalf("legacy %+v %+v %v", va, vb, e)
	}
	x, _ := models.GetCrashWork(db, av1.ID, "cursemark")
	y, _ := models.GetCrashWork(db, av2.ID, "cursemark")
	if x.Key != y.Key || x.Family != "access_violations" {
		t.Fatal("AV family split")
	}
	x, _ = models.GetCrashWork(db, l1.ID, "cursemark")
	y, _ = models.GetCrashWork(db, l2.ID, "cursemark")
	if x.Key == y.Key {
		t.Fatal("unknown legacy identities guessed")
	}
	x, _ = models.GetCrashWork(db, other.ID, "necrovale")
	if x.Key == va.Key {
		t.Fatal("project cross talk")
	}
	if _, e := models.GetCrashWork(db, a.ID, "necrovale"); !errors.Is(e, gorm.ErrRecordNotFound) {
		t.Fatal(e)
	}
	idx, e := models.ListCrashWork(db, "cursemark")
	if e != nil || len(idx.ActiveGroups) != 4 || idx.ActiveCount != 4 || idx.UnitType != "crash_group" {
		t.Fatalf("index %+v %v", idx, e)
	}
	var units int64
	db.Model(&models.CrashInvestigationUnit{}).Count(&units)
	if units != 0 {
		t.Fatal("read created units")
	}
	c, e := models.ClaimCrashWork(db, a.ID, "cursemark", "legacy-run", "")
	if e != nil || c.Acquired {
		t.Fatalf("already reviewed %+v %v", c, e)
	}
	if snapshot() != before {
		t.Fatal("migration/read/claim rewrote original evidence or statuses")
	}
	t.Setenv("MCP_UPSTREAM_TOKEN", strings.Repeat("a", 32))
	r := gin.New()
	RegisterMCPRoutes(r)
	w := noteRequest(r, "GET", fmt.Sprintf("report-investigations?kind=crash&project=cursemark&ids=%d,%d", a.ID, b.ID), "")
	var status struct{ Items []models.CrashWorkView }
	json.Unmarshal(w.Body.Bytes(), &status)
	if w.Code != 200 || len(status.Items) != 1 {
		t.Fatalf("per-report status workflow persists: %d %s", w.Code, w.Body)
	}
	w = noteRequest(r, "GET", "crashes", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"unit_type":"crash_group"`) {
		t.Fatal(w.Body)
	}
}
func TestCrashWorkConcurrentClaimsAliasRetryAndNewEvidence(t *testing.T) {
	db := workDB(t)
	a := workReport(t, db, "cursemark", "one", "fault", false)
	b := workReport(t, db, "cursemark", "one", "fault", false)
	var wg sync.WaitGroup
	results := make(chan models.CrashClaimResult, 8)
	errs := make(chan error, 8)
	keys := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := a.ID
			if i%2 == 1 {
				id = b.ID
			}
			k := fmt.Sprintf("claim-%03d", i)
			v, e := models.ClaimCrashWork(db, id, "cursemark", k, "")
			if e != nil {
				errs <- e
			}
			if v.Acquired {
				results <- v
				keys <- k
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
	if len(results) != 1 {
		t.Fatalf("duplicate workers granted: %d", len(results))
	}
	winner, key := <-results, <-keys
	replay, e := models.ClaimCrashWork(db, b.ID, "cursemark", key, "")
	if e != nil || !replay.Acquired || replay.Token != winner.Token {
		t.Fatalf("claim replay %+v %v", replay, e)
	}
	renewed, e := models.ClaimCrashWork(db, a.ID, "cursemark", key, winner.Token)
	if e != nil || !renewed.Acquired || !renewed.ExpiresAt.After(*winner.ExpiresAt) {
		t.Fatalf("renew %+v %v", renewed, e)
	}
	if _, e := models.ClaimCrashWork(db, a.ID, "cursemark", key, "wrong-token"); !errors.Is(e, models.ErrCrashClaim) {
		t.Fatal("forged renewal", e)
	}
	if _, _, e := models.AppendCrashWork(db, a.ID, "cursemark", "mcp", models.AppendCrashWorkInput{RequestKey: "no-claim", Note: "context"}); !errors.Is(e, models.ErrCrashClaim) {
		t.Fatal("unclaimed work", e)
	}
	if _, _, e := models.AppendReportNote(db, "crash", a.ID, "cursemark", "mcp", models.AppendReportNoteInput{RequestKey: "per-report", Note: "new investigation"}); !errors.Is(e, models.ErrCrashGroupRequired) {
		t.Fatal("per-report write allowed", e)
	}
	newer := workReport(t, db, "cursemark", "one", "fault", false)
	in := models.AppendCrashWorkInput{RequestKey: "group-outcome", Note: "Tested local change, not released.", State: "ready_for_review", ReleaseState: "local", ClaimToken: winner.Token, Complete: true}
	note, rep, e := models.AppendCrashWork(db, a.ID, "cursemark", "mcp", in)
	if e != nil || rep {
		t.Fatal(e)
	}
	note2, rep, e := models.AppendCrashWork(db, b.ID, "cursemark", "mcp", in)
	if e != nil || !rep || note2.ID != note.ID {
		t.Fatal("alias completion replay", e)
	}
	in.Note = "different"
	if _, _, e := models.AppendCrashWork(db, b.ID, "cursemark", "mcp", in); !errors.Is(e, models.ErrNoteConflict) {
		t.Fatal("changed replay", e)
	}
	v, e := models.GetCrashWork(db, newer.ID, "cursemark")
	if e != nil || !v.NeedsReview || v.State != "needs_confirmation" || v.CoveredThroughID != b.ID {
		t.Fatalf("arrivals were acknowledged too far %+v %v", v, e)
	}
	again, e := models.ClaimCrashWork(db, a.ID, "cursemark", "next-evidence", "")
	if e != nil || !again.Acquired || again.Investigation.Key != winner.Investigation.Key {
		t.Fatal(e)
	}
	workAppend(t, db, b, again.Token, "next-outcome", "blocked")
	done, e := models.ClaimCrashWork(db, a.ID, "cursemark", "no-repeat-run", "")
	if e != nil || done.Acquired {
		t.Fatal("duplicate finished group work", e)
	}
	old, e := models.ClaimCrashWork(db, a.ID, "cursemark", key, "")
	if e != nil || old.Acquired {
		t.Fatal("completed claim restarted", e)
	}
	log, e := models.ReadCrashInvestigationLog(db, b.ID, "cursemark", "", 0, 50)
	if e != nil || len(log.Items) != 2 || log.Items[0].Scope != "group" || log.Items[0].ReportID != 0 {
		t.Fatalf("group note ownership %+v %v", log, e)
	}
	var units, notes int64
	db.Model(&models.CrashInvestigationUnit{}).Count(&units)
	db.Model(&models.ReportNote{}).Where("kind='crashgrp'").Count(&notes)
	if units != 1 || notes != 2 {
		t.Fatalf("duplicate units/notes %d/%d", units, notes)
	}
}
func TestCrashWorkReturnExpiryManualRaceAndLegacyTime(t *testing.T) {
	db := workDB(t)
	a := workReport(t, db, "cursemark", "return", "fault", false)
	b := workReport(t, db, "cursemark", "return", "fault", false)
	claim, e := models.ClaimCrashWork(db, a.ID, "cursemark", "first-run", "")
	if e != nil || !claim.Acquired {
		t.Fatal(e)
	}
	workAppend(t, db, a, claim.Token, "first-outcome", "ready_for_review")
	if _, e := models.ResolveCrashID(db, uint64(a.ID)); e != nil {
		t.Fatal(e)
	}
	closed, _ := models.ClaimCrashWork(db, b.ID, "cursemark", "closed-run", "")
	if closed.Acquired {
		t.Fatal("closed group claimed")
	}
	newer := workReport(t, db, "cursemark", "return", "fault", false)
	v, e := models.GetCrashWork(db, newer.ID, "cursemark")
	if e != nil || !v.Returned || !v.NeedsReview || v.Key != claim.Investigation.Key || v.State != "needs_confirmation" {
		t.Fatalf("returned %+v %v", v, e)
	}
	returned, e := models.ClaimCrashWork(db, newer.ID, "cursemark", "returned-run", "")
	if e != nil || !returned.Acquired {
		t.Fatal(e)
	}
	if _, _, e := models.AppendCrashWork(db, newer.ID, "cursemark", "admin", models.AppendCrashWorkInput{RequestKey: "owner-context", Note: "New owner context."}); e != nil {
		t.Fatal(e)
	}
	if _, _, e := models.AppendCrashWork(db, newer.ID, "cursemark", "mcp", models.AppendCrashWorkInput{RequestKey: "stale-owner-note", Note: "stale result", State: "blocked", ClaimToken: returned.Token, Complete: true}); !errors.Is(e, models.ErrCrashClaimConflict) {
		t.Fatal("manual race silently overwritten", e)
	}
	if _, _, e := models.AppendCrashWork(db, newer.ID, "cursemark", "admin", models.AppendCrashWorkInput{RequestKey: "owner-recheck", Note: "Please investigate again.", State: "investigating"}); e != nil {
		t.Fatal(e)
	}
	resumed, e := models.ClaimCrashWork(db, newer.ID, "cursemark", "resumed-run", "")
	if e != nil || !resumed.Acquired {
		t.Fatal("owner recheck", e)
	}
	past := time.Now().UTC().Add(-time.Hour)
	db.Model(&models.CrashInvestigationUnit{}).Where("`key`=?", v.Key).Update("lease_until", past)
	db.Model(&models.CrashInvestigationClaim{}).Where("token=?", resumed.Token).Update("expires_at", past)
	expired, e := models.ClaimCrashWork(db, a.ID, "cursemark", "resumed-run", "")
	if e != nil || expired.Acquired {
		t.Fatal("expired replay renewed", e)
	}
	replacement, e := models.ClaimCrashWork(db, b.ID, "cursemark", "replacement-run", "")
	if e != nil || !replacement.Acquired {
		t.Fatal(e)
	}
	if _, _, e := models.AppendCrashWork(db, a.ID, "cursemark", "mcp", models.AppendCrashWorkInput{RequestKey: "expired-note", Note: "stale", ClaimToken: resumed.Token}); !errors.Is(e, models.ErrCrashClaim) {
		t.Fatal("expired token accepted", e)
	}
	// A manual closure/return during a claim invalidates the old lifecycle result.
	if _, e := models.ResolveCrashID(db, uint64(newer.ID)); e != nil {
		t.Fatal(e)
	}
	latest := workReport(t, db, "cursemark", "return", "fault", false)
	if _, _, e := models.AppendCrashWork(db, latest.ID, "cursemark", "mcp", models.AppendCrashWorkInput{RequestKey: "stale-cycle", Note: "old lifecycle", State: "ready_for_review", ClaimToken: replacement.Token, Complete: true}); !errors.Is(e, models.ErrCrashClaimConflict) {
		t.Fatal("old cycle overwrite", e)
	}
	var preserved models.Crash
	db.First(&preserved, a.ID)
	if !preserved.Resolved || preserved.Stack != "original evidence" {
		t.Fatal("history changed")
	}
	legacy := workReport(t, db, "cursemark", "legacy-time", "historical", true)
	db.Model(&legacy).Updates(map[string]any{"resolved_at": time.Now().Add(-time.Hour), "occurrence_version": 0, "ingested_at": nil})
	oldOpen := workReport(t, db, "cursemark", "legacy-time", "historical", false)
	db.Model(&oldOpen).Updates(map[string]any{"occurrence_version": 0, "ingested_at": nil})
	unknown, e := models.GetCrashWork(db, oldOpen.ID, "cursemark")
	if e != nil || unknown.Returned {
		t.Fatal("invented legacy recurrence time", e)
	}
}
